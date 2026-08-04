package database_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hkizilbulak/haradan-be/internal/platform/database"
)

func TestOpenInvalidURL(t *testing.T) {
	_, err := database.Open(context.Background(), database.Config{
		DatabaseURL:   "://bad",
		MaxConns:      2,
		MinConns:      1,
		HealthTimeout: time.Second,
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if strings.Contains(strings.ToLower(err.Error()), "password") {
		t.Fatalf("error leaked sensitive content: %v", err)
	}
}

func TestOpenEmptyURL(t *testing.T) {
	_, err := database.Open(context.Background(), database.Config{})
	if err == nil {
		t.Fatal("expected empty url error")
	}
}

func TestNilCloseAndPingSafety(t *testing.T) {
	var p *database.Postgres
	p.Close()
	if err := p.Ping(context.Background()); err == nil {
		t.Fatal("expected ping error on nil receiver")
	}
}
