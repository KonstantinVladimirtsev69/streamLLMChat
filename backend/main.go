package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"backend/internal/auth"
	"backend/internal/database"
	"backend/internal/llm"
	"backend/internal/repository/mongodb"
	"backend/internal/repository/postgres"
	"backend/internal/server"
	"backend/internal/server/handler"
	"backend/internal/service"

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

	srv := server.New()

	var mongoClient *mongo.Client
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

			// Initialize Auth & Referral dependencies
			jwtSecret := os.Getenv("JWT_SECRET")
			if len(jwtSecret) < 16 {
				jwtSecret = "default-insecure-dev-jwt-secret-at-least-32-chars!"
				log.Println("Warning: JWT_SECRET is empty or too short, using dev fallback")
			}

			tokenManager, err := auth.NewJWTTokenManager(jwtSecret)
			if err != nil {
				log.Fatalf("Failed to initialize token manager: %v", err)
			}

			frontendURL := os.Getenv("FRONTEND_URL")
			if frontendURL == "" {
				frontendURL = "http://localhost:3000"
			}

			userRepo := postgres.NewUserRepository(pgPool)
			balanceRepo := postgres.NewBalanceRepository(pgPool)
			referralRepo := postgres.NewReferralRepository(pgPool)
			txManager := database.NewTxManager(pgPool)
			refGen := auth.NewRefCodeGenerator(8)

			authService := service.NewAuthService(userRepo, balanceRepo, referralRepo, txManager, refGen, frontendURL)

			vkMock := os.Getenv("VK_MOCK_AUTH") == "true" || os.Getenv("VK_CLIENT_ID") == ""
			if vkMock {
				log.Println("VK Auth: MOCK mode active (VK_MOCK_AUTH=true or VK_CLIENT_ID is empty) -> redirects to /api/v1/auth/mock")
			} else {
				log.Printf("VK Auth: LIVE mode active (client_id=%s, redirect_uri=%s, base_url=%s)", os.Getenv("VK_CLIENT_ID"), os.Getenv("VK_REDIRECT_URI"), os.Getenv("VK_BASE_URL"))
			}
			vkClient := auth.NewVKOAuthClient(auth.VKConfig{
				ClientID:     os.Getenv("VK_CLIENT_ID"),
				ClientSecret: os.Getenv("VK_CLIENT_SECRET"),
				RedirectURI:  os.Getenv("VK_REDIRECT_URI"),
				BaseURL:      os.Getenv("VK_BASE_URL"),
				APIBaseURL:   os.Getenv("VK_API_URL"),
				FrontendURL:  frontendURL,
				MockAuth:     vkMock,
			})

			cookieSecure := os.Getenv("COOKIE_SECURE") == "true"
			cookieDomain := os.Getenv("COOKIE_DOMAIN")
			authHandler := handler.NewAuthHandler(authService, vkClient, tokenManager, handler.AuthHandlerConfig{
				FrontendURL:  frontendURL,
				CookieSecure: cookieSecure,
				CookieDomain: cookieDomain,
			})
			srv.RegisterAuthRoutes(authHandler, tokenManager)
			log.Println("Authentication and referral routes registered")

			// Initialize MongoDB repositories if MONGODB_URI or MONGO_URI is configured
			var chatService service.ChatService
			mongoURI := os.Getenv("MONGODB_URI")
			if mongoURI == "" {
				mongoURI = os.Getenv("MONGO_URI")
			}
			if mongoURI != "" {
				client, err := database.NewMongoClient(ctx, mongoURI)
				if err != nil {
					log.Printf("Warning: failed to connect to mongodb: %v", err)
				} else {
					mongoClient = client
					dbName := os.Getenv("MONGO_DB")
					if dbName == "" {
						if u, parseErr := url.Parse(mongoURI); parseErr == nil {
							path := strings.TrimPrefix(u.Path, "/")
							if path != "" {
								dbName = path
							}
						}
					}
					if dbName == "" {
						dbName = "llm_chat"
					}
					if err := database.EnsureIndexes(ctx, client.Database(dbName)); err != nil {
						log.Printf("Warning: failed to ensure mongodb indexes: %v", err)
					}
					log.Printf("MongoDB connection and indexes initialized (database: %s)", dbName)

					chatRepo := mongodb.NewChatRepository(client.Database(dbName))
					msgRepo := mongodb.NewMessageRepository(client.Database(dbName))
					chatService = service.NewChatService(chatRepo, msgRepo)
				}
			}

			// Initialize LLM Provider & Chat routes
			var llmProvider llm.Provider
			routerAIKey := os.Getenv("ROUTERAI_API_KEY")
			routerAIBaseURL := os.Getenv("ROUTERAI_BASE_URL")
			if routerAIBaseURL == "" {
				routerAIBaseURL = "https://routerai.ru/api/v1"
			}

			if routerAIKey == "" || os.Getenv("ROUTERAI_MOCK") == "true" {
				log.Println("ROUTERAI_API_KEY is not set or ROUTERAI_MOCK=true; using MockLLMProvider")
				llmProvider = llm.NewMockLLMProvider()
			} else {
				log.Printf("Initializing RouterAIClient with base URL: %s", routerAIBaseURL) //nolint:gosec
				llmProvider = llm.NewRouterAIClient(routerAIKey, routerAIBaseURL, &http.Client{Timeout: 60 * time.Second})
			}

			billingService := service.NewBillingService(balanceRepo)
			modelCache := llm.NewModelCache(llmProvider, 15*time.Minute)
			chatHandler := handler.NewChatHandler(llmProvider, modelCache, billingService, chatService)
			srv.RegisterChatRoutes(chatHandler, tokenManager)
			log.Println("LLM models catalog, chat session CRUD, and streaming routes registered")
		}
	}

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
