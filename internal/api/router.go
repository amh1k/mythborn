// Package api provides common server wiring and route registration. Feature
// routes are supplied by the HTTP API implementation.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/storage"
)

type Dependencies struct {
	DB                 contracts.Database
	Auth               auth.Authenticator
	Photos             storage.PhotoStorage
	Logger             *slog.Logger
	CORSAllowedOrigins []string
}

// RouteRegistrar adds product routes to the shared Go 1.26 ServeMux. Feature
// handlers should apply auth.Middleware to routes that require a session.
type RouteRegistrar interface {
	RegisterRoutes(*http.ServeMux, Dependencies)
}

type RouteRegistrarFunc func(*http.ServeMux, Dependencies)

func (f RouteRegistrarFunc) RegisterRoutes(mux *http.ServeMux, deps Dependencies) {
	f(mux, deps)
}

func NewHandler(deps Dependencies, registrars ...RouteRegistrar) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if deps.DB == nil || deps.DB.Ping(r.Context()) != nil {
			writeAPIError(w, http.StatusServiceUnavailable, "not_ready")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	for _, registrar := range registrars {
		if registrar != nil {
			registrar.RegisterRoutes(mux, deps)
		}
	}
	return cors(deps.CORSAllowedOrigins, mux)
}

func cors(origins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		if value := strings.TrimSpace(origin); value != "" {
			allowed[value] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := allowed[origin]; !ok {
			if r.Method == http.MethodOptions {
				writeAPIError(w, http.StatusForbidden, "origin_not_allowed")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
		w.Header().Set("Access-Control-Max-Age", "600")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeAPIError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "code": code})
}
