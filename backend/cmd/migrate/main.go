package main

import (
	"flag"
	"log"
	"os"

	"backend/internal/database"
)

func main() {
	action := flag.String("action", "up", "migration action: up or down")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	switch *action {
	case "up":
		if err := database.Up(dbURL); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}
		log.Println("Migrations applied successfully")
	case "down":
		if err := database.Down(dbURL); err != nil {
			log.Fatalf("Migration down failed: %v", err)
		}
		log.Println("Migration rolled back successfully")
	default:
		log.Fatalf("Unknown action: %s (expected 'up' or 'down')", *action)
	}
}
