package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Address, DatabaseURL, MongoURI, MongoDatabase, JWTSecret, Origin, UploadDir string
	SecureCookies                                                               bool
	MaxConnections                                                              int32
	MoodleURL                                                                   string
}

func Load() (Config, error) {
	c := Config{Address: env("HTTP_ADDR", "127.0.0.1:8080"), DatabaseURL: os.Getenv("DATABASE_URL"), MongoURI: os.Getenv("MONGODB_URI"), MongoDatabase: env("MONGODB_DATABASE", "mgimo"), JWTSecret: os.Getenv("JWT_SECRET"), Origin: env("APP_ORIGIN", "http://localhost:5173"), UploadDir: env("UPLOAD_DIR", "uploads"), SecureCookies: env("COOKIE_SECURE", "true") == "true", MaxConnections: 5}
	if c.DatabaseURL == "" {
		return c, errors.New("задайте DATABASE_URL")
	}
	if len(c.JWTSecret) < 32 {
		return c, errors.New("JWT_SECRET должен содержать минимум 32 символа")
	}
	u, e := url.Parse(c.Origin)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" {
		return c, errors.New("APP_ORIGIN должен быть origin без пути")
	}
	if !c.SecureCookies && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1")) {
		return c, errors.New("COOKIE_SECURE=false разрешён только локально")
	}
	if n := os.Getenv("DB_MAX_CONNECTIONS"); n != "" {
		v, e := strconv.Atoi(n)
		if e != nil || v < 1 || v > 20 {
			return c, errors.New("DB_MAX_CONNECTIONS: допустимо 1–20")
		}
		c.MaxConnections = int32(v)
	}
	if strings.Contains(c.DatabaseURL, "sslmode=disable") && u.Scheme == "https" {
		return c, errors.New("для публичного окружения требуется TLS PostgreSQL")
	}
	c.MoodleURL = env("MOODLE_URL", DefaultMoodleURL)
	if !ValidSubmissionURL(c.MoodleURL) {
		return c, errors.New("MOODLE_URL: ожидается HTTPS-ссылка без логина/пароля")
	}
	return c, nil
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
