// Command manage runs explicitly requested database maintenance.
// Use a direct PostgreSQL connection or a session pooler, not a transaction pooler:
// the advisory lock belongs to one physical session for the entire command.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const manageLock int64 = 0x4d47494d4f4442

type migration struct {
	version        int64
	name, checksum string
	statements     []string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		// Connection errors can contain DATABASE_URL credentials. Do not print them.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, env func(string) string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("использование: manage migrate [-dir migrations] | manage seed")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := "migrations"
	switch args[0] {
	case "migrate":
		fs.StringVar(&dir, "dir", dir, "каталог SQL-миграций относительно текущей папки")
	case "seed":
	default:
		return errors.New("допустимы команды migrate и seed")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("параметры команды: %w", err)
	}
	if fs.NArg() != 0 {
		return errors.New("лишние позиционные аргументы")
	}
	url := env("DATABASE_URL")
	if strings.TrimSpace(url) == "" {
		return errors.New("DATABASE_URL обязательна")
	}
	var files []migration
	var err error
	if args[0] == "seed" {
		if err = validateSeed(env); err != nil {
			return err
		}
	} else if files, err = loadMigrations(dir); err != nil {
		return err
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return errors.New("некорректная DATABASE_URL")
	}
	config.MaxConns = 1
	config.ConnConfig.ConnectTimeout = 10 * time.Second
	config.ConnConfig.RuntimeParams["search_path"] = "public, pg_catalog"
	config.ConnConfig.RuntimeParams["standard_conforming_strings"] = "on"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return errors.New("не удалось создать подключение PostgreSQL")
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return errors.New("не удалось подключиться к PostgreSQL; проверьте DATABASE_URL и доступность сервера")
	}
	defer conn.Release()
	if _, err = conn.Exec(ctx, "SELECT pg_catalog.pg_advisory_lock($1)", manageLock); err != nil {
		return errors.New("не удалось получить блокировку обслуживания PostgreSQL")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		if err := conn.QueryRow(cleanup, "SELECT pg_catalog.pg_advisory_unlock($1)", manageLock).Scan(&unlocked); err != nil || !unlocked {
			_ = conn.Conn().Close(cleanup) // Never return a session with a held lock to the pool.
		}
	}()
	if args[0] == "migrate" {
		return migrate(ctx, conn, files, out)
	}
	if err = seed(ctx, conn, env("DEMO_PASSWORD")); err != nil {
		return err
	}
	fmt.Fprintln(out, "Демонстрационные данные добавлены; существующие записи сохранены.")
	return nil
}

func validateSeed(env func(string) string) error {
	if env("ALLOW_DEMO_SEED") != "true" {
		return errors.New("seed разрешён только при ALLOW_DEMO_SEED=true")
	}
	p := env("DEMO_PASSWORD")
	if !utf8.ValidString(p) || utf8.RuneCountInString(p) < 12 || strings.TrimSpace(p) == "" {
		return errors.New("DEMO_PASSWORD должен содержать минимум 12 символов")
	}
	return nil
}

var migrationName = regexp.MustCompile(`^([0-9]+)_[A-Za-z0-9_]+\.sql$`)

func loadMigrations(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("чтение каталога миграций: %w", err)
	}
	var files []migration
	versions := map[int64]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		match := migrationName.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, fmt.Errorf("неверное имя миграции %s; ожидается 001_name.sql", entry.Name())
		}
		v, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || v < 1 || versions[v] {
			return nil, fmt.Errorf("неверная или повторяющаяся версия: %s", entry.Name())
		}
		versions[v] = true
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("чтение %s: %w", entry.Name(), err)
		}
		statements, err := migrationStatements(string(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		sum := sha256.Sum256(raw)
		files = append(files, migration{v, entry.Name(), hex.EncodeToString(sum[:]), statements})
	}
	if len(files) == 0 {
		return nil, errors.New("SQL-миграции не найдены")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}

func migrate(ctx context.Context, conn *pgxpool.Conn, files []migration, out io.Writer) error {
	_, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.schema_migrations (
 version bigint PRIMARY KEY, name text NOT NULL UNIQUE,
 checksum text NOT NULL CHECK (checksum ~ '^[0-9a-f]{64}$'),
 applied_at timestamptz NOT NULL DEFAULT now());
 ALTER TABLE public.schema_migrations ENABLE ROW LEVEL SECURITY;
 REVOKE ALL ON TABLE public.schema_migrations FROM PUBLIC;
 DO $$ DECLARE r text; BEGIN
 FOREACH r IN ARRAY ARRAY['anon','authenticated'] LOOP
 IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname=r) THEN
 EXECUTE format('REVOKE ALL ON TABLE public.schema_migrations FROM %I',r);
 END IF; END LOOP; END $$;`)
	if err != nil {
		return databaseError("создание журнала миграций", err)
	}
	rows, err := conn.Query(ctx, "SELECT version,name,checksum FROM public.schema_migrations ORDER BY version")
	if err != nil {
		return databaseError("чтение журнала миграций", err)
	}
	applied := map[int64]migration{}
	var latest int64
	for rows.Next() {
		var m migration
		if err = rows.Scan(&m.version, &m.name, &m.checksum); err != nil {
			rows.Close()
			return databaseError("чтение журнала", err)
		}
		applied[m.version] = m
		latest = m.version
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return databaseError("чтение журнала", err)
	}
	// Validate the entire history before applying ANY pending migration.
	available := map[int64]migration{}
	for _, m := range files {
		available[m.version] = m
	}
	for v, a := range applied {
		m, ok := available[v]
		if !ok {
			return fmt.Errorf("применённая миграция %s отсутствует в каталоге", a.name)
		}
		if a.name != m.name || a.checksum != m.checksum {
			return fmt.Errorf("несоответствие имени или SHA256 checksum миграции %s; применение остановлено", m.name)
		}
	}
	for _, m := range files {
		if _, ok := applied[m.version]; !ok && m.version < latest {
			return fmt.Errorf("миграция %s добавлена раньше уже применённой версии", m.name)
		}
	}
	for _, m := range files {
		if _, ok := applied[m.version]; ok {
			continue
		}
		if err = applyMigration(ctx, conn, m); err != nil {
			return err
		}
		fmt.Fprintf(out, "Применена %s\n", m.name)
	}
	fmt.Fprintln(out, "Миграции проверены; схема актуальна.")
	return nil
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, m migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return databaseError("начало транзакции", err)
	}
	defer rollback(tx)
	for i, statement := range m.statements {
		if _, err = tx.Exec(ctx, statement); err != nil {
			return databaseError(fmt.Sprintf("миграция %s, оператор %d", m.name, i+1), err)
		}
		if conn.Conn().PgConn().TxStatus() != 'T' {
			return fmt.Errorf("%s: оператор изменил состояние транзакции", m.name)
		}
	}
	if _, err = tx.Exec(ctx, "INSERT INTO public.schema_migrations(version,name,checksum) VALUES ($1,$2,$3)", m.version, m.name, m.checksum); err != nil {
		return databaseError("запись контрольной суммы", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return databaseError("фиксация миграции", err)
	}
	return nil
}

func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}

func databaseError(action string, err error) error {
	// Do not leak query parameters, passwords, passport hashes or connection URLs.
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		return fmt.Errorf("%s: PostgreSQL SQLSTATE %s", action, state.SQLState())
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: операция прервана", action)
	}
	return fmt.Errorf("%s: ошибка PostgreSQL", action)
}

// Split only at SQL statement boundaries, keeping dollar-quoted DO/function
// bodies, quoted identifiers, E-strings and nested comments intact. Comments
// become whitespace so transaction commands cannot hide behind them.
func splitSQL(sql string) ([]string, error) {
	var statements []string
	var b strings.Builder
	flush := func() {
		if s := strings.TrimSpace(b.String()); s != "" {
			statements = append(statements, s)
		}
		b.Reset()
	}
	for i := 0; i < len(sql); {
		switch {
		case strings.HasPrefix(sql[i:], "--"):
			j := strings.IndexByte(sql[i:], '\n')
			if j < 0 {
				i = len(sql)
			} else {
				i += j + 1
			}
			b.WriteByte(' ')
		case strings.HasPrefix(sql[i:], "/*"):
			depth := 1
			i += 2
			for i < len(sql) && depth > 0 {
				if strings.HasPrefix(sql[i:], "/*") {
					depth++
					i += 2
				} else if strings.HasPrefix(sql[i:], "*/") {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth != 0 {
				return nil, errors.New("незакрытый SQL-комментарий")
			}
			b.WriteByte(' ')
		case sql[i] == '\'' || sql[i] == '"':
			start := i
			quote := sql[i]
			escape := quote == '\'' && i > 0 && (sql[i-1] == 'e' || sql[i-1] == 'E') && (i < 2 || !identifierByte(sql[i-2]))
			i++
			closed := false
			for i < len(sql) {
				if escape && sql[i] == '\\' {
					i += 2
					continue
				}
				if sql[i] == quote {
					i++
					if i < len(sql) && sql[i] == quote {
						i++
						continue
					}
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, errors.New("незакрытая SQL-строка или идентификатор")
			}
			b.WriteString(sql[start:i])
		case sql[i] == '$' && (i == 0 || !identifierByte(sql[i-1])):
			j := i + 1
			for j < len(sql) && ((sql[j] >= 'a' && sql[j] <= 'z') || (sql[j] >= 'A' && sql[j] <= 'Z') || sql[j] == '_' || (j > i+1 && sql[j] >= '0' && sql[j] <= '9')) {
				j++
			}
			if j < len(sql) && sql[j] == '$' {
				tag := sql[i : j+1]
				end := strings.Index(sql[j+1:], tag)
				if end < 0 {
					return nil, errors.New("незакрытая dollar-quoted SQL-строка")
				}
				end += j + 1 + len(tag)
				b.WriteString(sql[i:end])
				i = end
			} else {
				b.WriteByte(sql[i])
				i++
			}
		case sql[i] == ';':
			flush()
			i++
		default:
			b.WriteByte(sql[i])
			i++
		}
	}
	flush()
	return statements, nil
}
func identifierByte(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 128
}

func migrationStatements(sql string) ([]string, error) {
	statements, err := splitSQL(sql)
	if err != nil {
		return nil, err
	}
	if len(statements) == 0 {
		return nil, errors.New("пустая миграция")
	}
	if strings.EqualFold(statements[0], "BEGIN") {
		if len(statements) < 3 || !strings.EqualFold(statements[len(statements)-1], "COMMIT") {
			return nil, errors.New("ожидается одна внешняя пара BEGIN; ... COMMIT;")
		}
		statements = statements[1 : len(statements)-1]
	}
	for _, s := range statements {
		fields := strings.Fields(strings.ToUpper(s))
		word := fields[0]
		// Commands can be followed by punctuation without whitespace.
		for i, c := range word {
			if c < 'A' || c > 'Z' {
				word = word[:i]
				break
			}
		}
		switch word {
		case "BEGIN", "START", "COMMIT", "END", "ROLLBACK", "ABORT", "SAVEPOINT", "RELEASE", "PREPARE":
			return nil, errors.New("внутреннее управление транзакциями запрещено")
		case "SET", "RESET":
			if strings.Contains(strings.ToUpper(s), "STANDARD_CONFORMING_STRINGS") {
				return nil, errors.New("изменение разбора SQL-строк запрещено")
			}
		}
	}
	return statements, nil
}
