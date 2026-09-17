package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/hkizilbulak/haradan-be/internal/platform/database"
)

func main() {
	godotenv.Load(".env")
	godotenv.Load(".env.local")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("No DATABASE_URL")
		return
	}
	db, err := database.Open(context.Background(), database.Config{
		DatabaseURL: dbURL,
		MaxConns: 2,
		MinConns: 1,
	})
	if err != nil {
		fmt.Println("DB open err:", err)
		return
	}
	defer db.Close()
	
	// Query notifications
	rows, err := db.Pool().Query(context.Background(), `SELECT id, event_type, created_at FROM hrd_notifications WHERE event_type = 'ADVERT_PRICE_DROP' ORDER BY created_at DESC LIMIT 5`)
	if err != nil {
		fmt.Println("Query err:", err)
		return
	}
	defer rows.Close()
	fmt.Println("Recent Price Drop Notifications:")
	for rows.Next() {
		var id, et string
		var ca string
		if err := rows.Scan(&id, &et, &ca); err != nil {
			fmt.Println("Scan err:", err)
			continue
		}
		fmt.Printf("ID: %s, Type: %s, Created: %s\n", id, et, ca)
	}

	// Query user states
	rows2, err := db.Pool().Query(context.Background(), `SELECT notification_id, user_id, delivered_at, email_status FROM hrd_user_notification_states ORDER BY created_at DESC LIMIT 10`)
	if err != nil {
		fmt.Println("Query2 err:", err)
		return
	}
	defer rows2.Close()
	fmt.Println("\nRecent User Notification States:")
	for rows2.Next() {
		var nid, uid string
		var da, es *string
		if err := rows2.Scan(&nid, &uid, &da, &es); err != nil {
			fmt.Println("Scan2 err:", err)
			continue
		}
		
		fmt.Printf("NID: %s, UID: %s, EmailStatus: %v\n", nid, uid, es)
	// Query jobs
	rows3, err := db.Pool().Query(context.Background(), `SELECT id, job_type, status, last_error FROM hrd_background_jobs WHERE job_type = 'NOTIFICATION_FANOUT_ADVERT_PRICE_DROP' ORDER BY created_at DESC LIMIT 5`)
	if err != nil {
		fmt.Println("Query3 err:", err)
		return
	}
	defer rows3.Close()
	fmt.Println("\nRecent Fanout Jobs:")
	for rows3.Next() {
		var id, jt, status string
		var le *string
		if err := rows3.Scan(&id, &jt, &status, &le); err != nil {
			fmt.Println("Scan3 err:", err)
			continue
		}
		
		fmt.Printf("JobID: %s, Type: %s, Status: %s, LastError: %v\n", id, jt, status, le)
	// Query templates
	rows4, err := db.Pool().Query(context.Background(), `SELECT id, event_type, status FROM hrd_notification_templates WHERE event_type = 'ADVERT_PRICE_DROP'`)
	if err != nil {
		fmt.Println("Query4 err:", err)
		return
	}
	defer rows4.Close()
	fmt.Println("\nTemplates:")
	for rows4.Next() {
		var id, et, status string
		if err := rows4.Scan(&id, &et, &status); err != nil {
			continue
		}
		fmt.Printf("TemplateID: %s, Type: %s, Status: %s\n", id, et, status)
	}
}
