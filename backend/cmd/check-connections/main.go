// Read-only checks. Never print connection strings or underlying driver errors.
package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type result struct {
	Service   string `json:"service"`
	Connected bool   `json:"connected"`
	Objects   int    `json:"project_objects"`
	Message   string `json:"message"`
}

func classify(err error) string {
	var dns *net.DNSError
	var cert x509.UnknownAuthorityError
	var hostname x509.HostnameError
	if errors.As(err, &dns) {
		return "Не удалось разрешить адрес сервиса через DNS"
	}
	if errors.As(err, &cert) || errors.As(err, &hostname) {
		return "Не пройдена проверка сертификата TLS"
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "authentication") || strings.Contains(s, "password") || strings.Contains(s, "sasl") || strings.Contains(s, "scram") {
		return "Сервис отклонил учётные данные или способ аутентификации"
	}
	if strings.Contains(s, "certificate") || strings.Contains(s, "tls") {
		return "Ошибка защищённого соединения TLS"
	}
	if strings.Contains(s, "timeout") || strings.Contains(s, "deadline") {
		return "Истекло время подключения; проверьте доступ сети и список разрешённых IP"
	}
	return "Проверка подключения не выполнена; параметры и сетевой доступ требуют проверки"
}

func postgres() result {
	r := result{Service: "PostgreSQL"}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cfg, e := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if e != nil {
		r.Message = "Некорректный DATABASE_URL"
		return r
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.ConnectTimeout = 15 * time.Second
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		r.Message = classify(e)
		return r
	}
	defer db.Close()
	if e = db.Ping(ctx); e != nil {
		r.Message = classify(e)
		return r
	}
	r.Connected = true
	e = db.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_tables WHERE schemaname='public' AND tablename=ANY($1::text[])`, []string{"accounts", "students", "teachers", "employers", "lessons", "appointment", "courses", "user_sessions"}).Scan(&r.Objects)
	if e != nil {
		r.Message = "Подключение установлено, чтение каталога схемы не выполнено"
		return r
	}
	r.Message = "Подключение установлено; выполнено только чтение каталога"
	return r
}

func mongodb() result {
	r := result{Service: "MongoDB"}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		r.Message = "MONGODB_URI не задан"
		return r
	}
	client, e := mongo.Connect(options.Client().ApplyURI(uri).SetMaxPoolSize(1).SetServerSelectionTimeout(15 * time.Second))
	if e != nil {
		r.Message = classify(e)
		return r
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
	}()
	if e = client.Ping(ctx, nil); e != nil {
		r.Message = classify(e)
		return r
	}
	r.Connected = true
	name := os.Getenv("MONGODB_DATABASE")
	if name == "" {
		name = "mgimo"
	}
	names, e := client.Database(name).ListCollectionNames(ctx, map[string]any{})
	if e != nil {
		r.Message = "Подключение установлено, чтение коллекций недоступно: проверьте права пользователя на выбранную базу"
		return r
	}
	for _, n := range names {
		if n == "message_conversations" || n == "private_messages" {
			r.Objects++
		}
	}
	r.Message = "Подключение установлено; выполнено только чтение списка коллекций"
	return r
}

func main() {
	var p, m result
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); p = postgres() }()
	go func() { defer wg.Done(); m = mongodb() }()
	wg.Wait()
	_ = json.NewEncoder(os.Stdout).Encode([]result{p, m})
	if !p.Connected || !m.Connected {
		os.Exit(1)
	}
}
