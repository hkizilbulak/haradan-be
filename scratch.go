package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	pgadmin "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/adminuser"
)

func main() {
	godotenv.Load(".env")
	dbURL := os.Getenv("DATABASE_URL")
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	ctx := context.Background()

	targetUserID := uuid.MustParse("6e7702c7-ae9b-4564-9d17-68fb6326f76e")
	var adminID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM hrd_users WHERE role = 'admin' LIMIT 1").Scan(&adminID)
	if err != nil {
		fmt.Println("Find admin error:", err)
		return
	}
	fmt.Println("Target user ID:", targetUserID)
	fmt.Println("Admin ID:", adminID)

	repo := pgadmin.NewRepository(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		panic(err)
	}
	defer tx.Rollback(ctx)

	txRepo := repo.WithTx(tx)
	err = txRepo.DeleteUser(ctx, targetUserID, adminID)
	fmt.Printf("DeleteUser error: %+v\n", err)
}

