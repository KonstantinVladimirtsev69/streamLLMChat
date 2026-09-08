package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"backend/internal/database"
	"backend/internal/server"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func main() {
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		if portNum, err := strconv.Atoi(p); err == nil && portNum > 0 && portNum <= 65535 {
			port = strconv.Itoa(portNum)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize PostgreSQL if DATABASE_URL is configured
	var pgPool *pgxpool.Pool
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		if err := database.Up(dbURL); err != nil {
			log.Printf("Warning: failed to auto-run migrations: %v", err)
		}
		pool, err := database.NewPostgresPool(ctx, dbURL)
		if err != nil {
			log.Printf("Warning: failed to connect to postgres: %v", err)
		} else {
			pgPool = pool
			log.Println("PostgreSQL connection pool initialized")
		}
	}

	// Initialize MongoDB if MONGODB_URI is configured
	var mongoClient *mongo.Client
	if mongoURI := os.Getenv("MONGODB_URI"); mongoURI != "" {
		client, err := database.NewMongoClient(ctx, mongoURI)
		if err != nil {
			log.Printf("Warning: failed to connect to mongodb: %v", err)
		} else {
			mongoClient = client
			dbName := os.Getenv("MONGO_DB")
			if dbName == "" {
				dbName = "llmchat"
			}
			if err := database.EnsureIndexes(ctx, client.Database(dbName)); err != nil {
				log.Printf("Warning: failed to ensure mongodb indexes: %v", err)
			}
			log.Println("MongoDB connection and indexes initialized")
		}
	}

	srv := server.New()

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      srv.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("Backend server listening on port %s", port)
		serverErrors <- httpServer.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("Received signal %v: initiating graceful shutdown", sig)

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Could not stop server gracefully: %v", err)
			if err := httpServer.Close(); err != nil {
				log.Fatalf("Could not force stop server: %v", err)
			}
		}

		if pgPool != nil {
			pgPool.Close()
			log.Println("PostgreSQL pool closed")
		}

		if mongoClient != nil {
			_ = mongoClient.Disconnect(shutdownCtx)
			log.Println("MongoDB client disconnected")
		}

		log.Println("Server gracefully stopped")
	}
}
