package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/google/uuid"
	
	"github.com/hkizilbulak/haradan-be/internal/platform/database"
	"github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/application/advert"
	"github.com/hkizilbulak/haradan-be/internal/application/notification"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/advert"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/notification"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/packaging"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/media"
)

func main() {
	godotenv.Load(".env")
	godotenv.Load(".env.local")
	dbURL := os.Getenv("DATABASE_URL")
	db, _ := database.Open(context.Background(), database.Config{DatabaseURL: dbURL, MaxConns: 2, MinConns: 1})
	defer db.Close()
	
	ownerID, _ := uuid.Parse("5e22921f-bf49-42ed-b7a5-4b2297096da8")
	advertID := int64(129) // ADA FENERİ
	
	notifRepo := pgnotif.NewRepository(db.Pool())
	advRepo := pgadvert.NewRepository(db.Pool())
	pkgRepo := pgpackaging.NewRepository(db.Pool())
	mediaRepo := pgmedia.NewRepository(db.Pool())
	
	writer := appnotif.NewEventWriter(
		appnotif.EventWriterConfig{
			Repo:     notifRepo,
			Adverts:  advRepo,
			Packages: pkgRepo,
			FrontendURL: "http://localhost:3000",
		},
	)
	emitter, _ := appnotif.NewEmitter(appnotif.EmitterConfig{
		Writer: writer, Adverts: advRepo, Packages: pkgRepo,
	})
	
	svc := appadvert.NewService(appadvert.ServiceConfig{
		Repo: advRepo, Media: mediaRepo, Packages: pkgRepo, DB: db.Pool(), Notifications: emitter,
	})
	
	price := int64(40000000)
	_, err := svc.UpdateAdvertDraftDetails(context.Background(), ownerID, advertID, appadvert.UpdateDetailsInput{
		ExpectedVersion: 30, // ADA FENERİ version... let's just pass expectedVersion that matches. Wait, if we don't match, it fails.
		PriceSet: true,
		Price: &appadvert.MoneyInput{AmountMinor: &price},
	})
	if err != nil {
		fmt.Printf("UpdateAdvertDraftDetails error: %v\n", err)
	} else {
		fmt.Println("Success! Price updated.")
	}
}
