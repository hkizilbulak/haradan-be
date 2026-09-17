package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/jackc/pgx/v5/pgxpool"
	pgnotif "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/notification"
)

func main() {
	godotenv.Load(".env")
	godotenv.Load(".env.local")
	dbURL := os.Getenv("DATABASE_URL")
	pool, _ := pgxpool.New(context.Background(), dbURL)
	defer pool.Close()
	
	repo := pgnotif.NewRepository(pool)
	
	u, _ := uuid.Parse("c7c58902-f99a-41aa-8725-c6a4c015fbf9")
	res, err := repo.ListUserNotifications(context.Background(), u, nil, nil, 20)
	fmt.Println("Result length:", len(res))
	if len(res) > 0 {
		fmt.Printf("Item: %+v\n", res[0])
	}
	fmt.Println("Error:", err)
}
