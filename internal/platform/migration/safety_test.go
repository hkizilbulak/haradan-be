package migration_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/hkizilbulak/haradan-be/internal/platform/migration"
	"github.com/hkizilbulak/haradan-be/migrations"
)

func TestValidateEmbeddedMigrationsPass(t *testing.T) {
	if err := migration.ValidateEmbeddedMigrations(migrations.FS); err != nil {
		t.Fatalf("expected embedded migrations to pass: %v", err)
	}
}

func TestValidateRejectsForbiddenPatterns(t *testing.T) {
	cases := map[string]string{
		"hr_ref": `-- +goose Up
CREATE TABLE hrd_users (id uuid PRIMARY KEY);
CREATE TABLE hrd_auth_sessions (id uuid PRIMARY KEY);
CREATE TABLE hrd_one_time_credentials (id uuid PRIMARY KEY);
CREATE TABLE hrd_security_events (id uuid PRIMARY KEY);
ALTER TABLE hr_legacy ADD COLUMN x int;
-- +goose Down
DROP TABLE hrd_security_events;
DROP TABLE hrd_one_time_credentials;
DROP TABLE hrd_auth_sessions;
DROP TABLE hrd_users;
`,
		"cascade": `-- +goose Up
CREATE TABLE hrd_users (id uuid PRIMARY KEY);
CREATE TABLE hrd_auth_sessions (id uuid PRIMARY KEY);
CREATE TABLE hrd_one_time_credentials (id uuid PRIMARY KEY);
CREATE TABLE hrd_security_events (id uuid PRIMARY KEY);
-- +goose Down
DROP TABLE hrd_security_events;
DROP TABLE hrd_one_time_credentials;
DROP TABLE hrd_auth_sessions;
DROP TABLE hrd_users CASCADE;
`,
		"if_not_exists": `-- +goose Up
CREATE TABLE IF NOT EXISTS hrd_users (id uuid PRIMARY KEY);
CREATE TABLE hrd_auth_sessions (id uuid PRIMARY KEY);
CREATE TABLE hrd_one_time_credentials (id uuid PRIMARY KEY);
CREATE TABLE hrd_security_events (id uuid PRIMARY KEY);
-- +goose Down
DROP TABLE hrd_security_events;
DROP TABLE hrd_one_time_credentials;
DROP TABLE hrd_auth_sessions;
DROP TABLE hrd_users;
`,
		"unprefixed_index": `-- +goose Up
CREATE TABLE hrd_users (id uuid PRIMARY KEY);
CREATE TABLE hrd_auth_sessions (id uuid PRIMARY KEY);
CREATE TABLE hrd_one_time_credentials (id uuid PRIMARY KEY);
CREATE TABLE hrd_security_events (id uuid PRIMARY KEY);
CREATE INDEX users_status_idx ON hrd_users (id);
-- +goose Down
DROP TABLE hrd_security_events;
DROP TABLE hrd_one_time_credentials;
DROP TABLE hrd_auth_sessions;
DROP TABLE hrd_users;
`,
		"missing_markers": `CREATE TABLE hrd_users (id uuid PRIMARY KEY);`,
	}

	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			fsys := cloneEmbeddedWithOverride(t, "00001_users_auth_security.sql", bad)
			if err := migration.ValidateEmbeddedMigrations(fsys); err == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
}

func TestValidateRejectsExtraAndMissingTables(t *testing.T) {
	extra := `-- +goose Up
CREATE TABLE hrd_users (id uuid PRIMARY KEY);
CREATE TABLE hrd_auth_sessions (id uuid PRIMARY KEY);
CREATE TABLE hrd_one_time_credentials (id uuid PRIMARY KEY);
CREATE TABLE hrd_security_events (id uuid PRIMARY KEY);
CREATE TABLE hrd_payments (id uuid PRIMARY KEY);
-- +goose Down
DROP TABLE hrd_payments;
DROP TABLE hrd_security_events;
DROP TABLE hrd_one_time_credentials;
DROP TABLE hrd_auth_sessions;
DROP TABLE hrd_users;
`
	fsys := cloneEmbeddedWithOverride(t, "00001_users_auth_security.sql", extra)
	if err := migration.ValidateEmbeddedMigrations(fsys); err == nil {
		t.Fatal("expected extra table failure")
	}

	meta := `-- +goose Up
CREATE TABLE hrd_schema_migrations (id int PRIMARY KEY);
CREATE TABLE hrd_users (id uuid PRIMARY KEY);
CREATE TABLE hrd_auth_sessions (id uuid PRIMARY KEY);
CREATE TABLE hrd_one_time_credentials (id uuid PRIMARY KEY);
CREATE TABLE hrd_security_events (id uuid PRIMARY KEY);
-- +goose Down
DROP TABLE hrd_security_events;
DROP TABLE hrd_one_time_credentials;
DROP TABLE hrd_auth_sessions;
DROP TABLE hrd_users;
DROP TABLE hrd_schema_migrations;
`
	fsys = cloneEmbeddedWithOverride(t, "00001_users_auth_security.sql", meta)
	if err := migration.ValidateEmbeddedMigrations(fsys); err == nil {
		t.Fatal("expected metadata create failure")
	}
}

func TestRunnerUnknownCommandAndDownGuard(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://localhost:1/haradan")
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	defer db.Close()

	r := &migration.Runner{DB: db, FS: migrations.FS}
	if err := r.Run(context.Background(), "wat"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected unknown command error, got %v", err)
	}

	_ = os.Unsetenv("ALLOW_DESTRUCTIVE_MIGRATIONS")
	if err := r.Run(context.Background(), "down"); err == nil || !strings.Contains(err.Error(), "ALLOW_DESTRUCTIVE_MIGRATIONS") {
		t.Fatalf("expected down guard error, got %v", err)
	}
}

func TestCanonicalMetadataTableConstant(t *testing.T) {
	if migration.SchemaMigrationsTable != "hrd_schema_migrations" {
		t.Fatalf("unexpected metadata table %q", migration.SchemaMigrationsTable)
	}
}

func cloneEmbeddedWithOverride(t *testing.T, overrideName, content string) fstest.MapFS {
	t.Helper()
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	out := make(fstest.MapFS)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		raw, err := migrations.FS.ReadFile(e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		out[e.Name()] = &fstest.MapFile{Data: raw}
	}
	out[overrideName] = &fstest.MapFile{Data: []byte(content)}
	return out
}
