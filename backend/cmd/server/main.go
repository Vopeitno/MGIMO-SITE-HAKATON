package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"mgimo/backend/internal/api"
	"mgimo/backend/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("сервер остановлен", "reason", err.Error())
		os.Exit(1)
	}
}
func run() error {
	c, e := config.Load()
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pg, e := pgxpool.ParseConfig(c.DatabaseURL)
	if e != nil {
		return errors.New("некорректный DATABASE_URL")
	}
	pg.MaxConns = c.MaxConnections
	pg.MinConns = 0
	pg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pg.ConnConfig.ConnectTimeout = 10 * time.Second
	db, e := pgxpool.NewWithConfig(ctx, pg)
	if e != nil {
		return errors.New("не удалось создать пул PostgreSQL")
	}
	defer db.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	e = db.Ping(pingCtx)
	cancel()
	if e != nil {
		return errors.New("PostgreSQL недоступен; проверьте настройки подключения")
	}
	var mongoDB *mongo.Database
	var client *mongo.Client
	if c.MongoURI != "" {
		client, e = mongo.Connect(options.Client().ApplyURI(c.MongoURI).SetMaxPoolSize(5).SetMinPoolSize(0).SetServerSelectionTimeout(5 * time.Second))
		if e == nil {
			pingCtx, cancel = context.WithTimeout(ctx, 6*time.Second)
			e = client.Ping(pingCtx, nil)
			cancel()
		}
		if e != nil {
			slog.Warn("MongoDB недоступна: сообщения отключены до перезапуска")
		} else {
			mongoDB = client.Database(c.MongoDatabase)
		}
		if client != nil {
			defer func() {
				shutdown, c := context.WithTimeout(context.Background(), 5*time.Second)
				defer c()
				_ = client.Disconnect(shutdown)
			}()
		}
	}
	app, e := api.New(db, mongoDB, c)
	if e != nil {
		return e
	}
	if mongoDB != nil {
		indexCtx, indexCancel := context.WithTimeout(ctx, 10*time.Second)
		e = app.InitMessages(indexCtx)
		indexCancel()
		if e != nil {
			return errors.New("не удалось подготовить индексы MongoDB")
		}
	}
	server := &http.Server{Addr: c.Address, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 * 1024}
	errs := make(chan error, 1)
	go func() {
		slog.Info("сервер запущен", "address", c.Address)
		errs <- server.ListenAndServe()
	}()
	select {
	case e = <-errs:
		if !errors.Is(e, http.ErrServerClosed) {
			return errors.New("не удалось запустить HTTP сервер")
		}
		return nil
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
