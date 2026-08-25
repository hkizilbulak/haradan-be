package catalog

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListAllPropertiesForYarisAti(t *testing.T) {
	dbURL := "postgres://postgres:mpUDvyjANKlQanVKHmxKCszeEXmWhqLy@reseau.proxy.rlwy.net:32596/railway?sslmode=disable"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `
		SELECT id, code, title, is_active, sort_order
		FROM hrd_category_properties
		WHERE category_id = 'c1000000-0000-4000-8000-000000000011'
		ORDER BY sort_order, title
	`)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== PROPERTIES IN DB FOR SATILIK YARIS ATI ===")
	for rows.Next() {
		var id, code, title string
		var isActive bool
		var sortOrder int
		if err := rows.Scan(&id, &code, &title, &isActive, &sortOrder); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		status := "🟢 AKTİF"
		if !isActive {
			status = "🔴 PASİF"
		}
		fmt.Printf("ID: %s | Status: %-10s | Code: %-25s | Title: %s\n", id, status, code, title)
	}
}
