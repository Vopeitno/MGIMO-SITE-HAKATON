package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationParser(t *testing.T) {
	sql := `BEGIN; -- comment
 CREATE TABLE x ("v;" text DEFAULT 'a;b');
 DO $body$ BEGIN PERFORM 'a;b'; END $body$;
 /* nested /* comment; */ retained */ COMMIT;`
	statements, e := migrationStatements(sql)
	if e != nil || len(statements) != 2 {
		t.Fatalf("%v %v", statements, e)
	}
	for _, bad := range []string{"BEGIN; SELECT 1;", "SELECT 1; COMMIT;", "SELECT 'unclosed", "DO $$ broken", "/* open", "BEGIN; SET standard_conforming_strings=off; COMMIT;"} {
		if _, e = migrationStatements(bad); e == nil {
			t.Errorf("принято %q", bad)
		}
	}
}
func TestLoadMigrationsChecksum(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "001_first.sql")
	if e := os.WriteFile(file, []byte("SELECT 1;"), 0600); e != nil {
		t.Fatal(e)
	}
	a, e := loadMigrations(dir)
	if e != nil || len(a) != 1 {
		t.Fatal(e)
	}
	if e = os.WriteFile(file, []byte("SELECT 2;"), 0600); e != nil {
		t.Fatal(e)
	}
	b, e := loadMigrations(dir)
	if e != nil || a[0].checksum == b[0].checksum {
		t.Fatal("checksum не изменился")
	}
}
func TestDemoSeedGuard(t *testing.T) {
	env := func(s string) string {
		if s == "DEMO_PASSWORD" {
			return strings.Repeat("p", 12)
		}
		return ""
	}
	if validateSeed(env) == nil {
		t.Fatal("разрешён неявный seed")
	}
	if validateSeed(func(s string) string {
		if s == "ALLOW_DEMO_SEED" {
			return "true"
		}
		return "short"
	}) == nil {
		t.Fatal("короткий пароль принят")
	}
}
