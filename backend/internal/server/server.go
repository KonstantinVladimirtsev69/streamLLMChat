package server

import (
	"encoding/json"
	"net/http"

	"backend/internal/auth"
	"backend/internal/server/handler"
	appMiddleware "backend/internal/server/middleware"

	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Server wraps the HTTP router and route configuration.
type Server struct {
	Router *chi.Mux
}

// New initializes and returns a new Server instance with configured middleware and routes.
func New() *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	s := &Server{
		Router: r,
	}

	s.routes()

	return s
}

func (s *Server) routes() {
	s.Router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})
}

// RegisterAuthRoutes registers all public and protected authentication endpoints.
func (s *Server) RegisterAuthRoutes(h *handler.AuthHandler, tokenManager auth.TokenManager) {
	s.Router.Route("/api/v1/auth", func(r chi.Router) {
		r.Get("/vk/login", h.Login)
		r.Get("/vk/callback", h.Callback)
		r.Get("/mock", h.MockLogin)
		r.Post("/logout", h.Logout)

		// Protected endpoints
		r.Group(func(pr chi.Router) {
			pr.Use(appMiddleware.AuthMiddleware(tokenManager))
			pr.Get("/me", h.Me)
		})
	})
}

// RegisterChatRoutes registers endpoints for models catalog, chat session CRUD, and streaming.
func (s *Server) RegisterChatRoutes(h *handler.ChatHandler, tokenManager auth.TokenManager) {
	s.Router.Route("/api/v1", func(r chi.Router) {
		r.Group(func(pr chi.Router) {
			pr.Use(appMiddleware.AuthMiddleware(tokenManager))
			pr.Get("/models", h.GetModels)
			pr.Post("/chat/stream", h.StreamChat)

			pr.Route("/chats", func(cr chi.Router) {
				cr.Post("/", h.CreateChat)
				cr.Get("/", h.ListChats)
				cr.Get("/{id}", h.GetChat)
				cr.Patch("/{id}", h.UpdateChatTitle)
				cr.Delete("/{id}", h.DeleteChat)
				cr.Get("/{id}/messages", h.ListMessages)
			})
		})
	})
}

// RegisterLLMRoutes is an alias to RegisterChatRoutes for backwards compatibility.
func (s *Server) RegisterLLMRoutes(h *handler.ChatHandler, tokenManager auth.TokenManager) {
	s.RegisterChatRoutes(h, tokenManager)
}
