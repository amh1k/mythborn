// Package auth defines the verified account identity passed through API
// request context. Supabase JWT verification is supplied by an authenticator.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/amh1k/mythborn/internal/domain"
)

var ErrUnauthorized = errors.New("unauthorized")

type Principal struct {
	AccountID  domain.ID
	SystemRole domain.SystemRole
}

func (p Principal) IsAdmin() bool { return p.SystemRole == domain.SystemRoleAdmin }

type Authenticator interface {
	Authenticate(context.Context, string) (Principal, error)
}

type contextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(Principal)
	return p, ok
}

// Middleware validates a bearer token and makes the verified Principal
// available to downstream handlers. Authorization decisions remain in the
// application/API layer and must keep game roles separate from admin access.
func Middleware(authenticator Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authenticator == nil {
			writeError(w, http.StatusServiceUnavailable, "authentication_unavailable")
			return
		}
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		principal, err := authenticator.Authenticate(r.Context(), parts[1])
		if err != nil || principal.AccountID == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
	})
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "code": code})
}
