package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := "postgres://haradan:haradan@localhost:5432/haradan?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	tag, err := pool.Exec(ctx, "DELETE FROM hrd_stud_farms WHERE first_name = '' OR last_name = ''")
	if err != nil {
		fmt.Printf("Error deleting: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted %d empty records.\n", tag.RowsAffected())
}
