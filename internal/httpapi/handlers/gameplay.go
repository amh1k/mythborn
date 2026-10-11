package handlers

import (
	"net/http"
	"strconv"

	"github.com/amh1k/mythborn/internal/api"
	"github.com/amh1k/mythborn/internal/app"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
)

func registerGameplayRoutes(mux *http.ServeMux, deps api.Dependencies, lore app.Lore) {
	wrap := func(fn func(http.ResponseWriter, *http.Request, auth.Principal, domain.ID)) http.Handler {
		return auth.Middleware(deps.Auth, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, key := range []string{"id", "agent", "mystery", "relationship", "tradition"} {
				if value := r.PathValue(key); value != "" && !validUUID(value) {
					writeError(w, app.ErrInvalid)
					return
				}
			}
			w.Header().Set("Cache-Control", "no-store")
			p, _ := auth.PrincipalFromContext(r.Context())
			fn(w, r, p, domain.ID(r.PathValue("id")))
		}))
	}
	respond := func(w http.ResponseWriter, v any, err error) {
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
	version := func(w http.ResponseWriter, r *http.Request) (int64, bool) {
		raw := r.URL.Query().Get("after_version")
		if raw == "" {
			return 0, true
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			writeError(w, app.ErrInvalid)
			return 0, false
		}
		return n, true
	}
	cursorOK := func(w http.ResponseWriter, cursor string) bool {
		if cursor != "" && !validUUID(cursor) {
			writeError(w, app.ErrInvalid)
			return false
		}
		return true
	}
	mux.Handle("GET /api/v1/worlds/{id}/mysteries", wrap(func(w http.ResponseWriter, r *http.Request, p auth.Principal, game domain.ID) {
		state := r.URL.Query().Get("state")
		cursor := r.URL.Query().Get("cursor")
		if state != "" && state != "open" && state != "resolved" {
			writeError(w, app.ErrInvalid)
			return
		}
		if !cursorOK(w, cursor) {
			return
		}
		v, e := lore.Mysteries(r.Context(), p, game, state, cursor)
		respond(w, v, e)
	}))
	mux.Handle("GET /api/v1/worlds/{id}/mysteries/{mystery}", wrap(func(w http.ResponseWriter, r *http.Request, p auth.Principal, game domain.ID) {
		after, ok := version(w, r)
		if !ok {
			return
		}
		v, e := lore.Mystery(r.Context(), p, game, domain.ID(r.PathValue("mystery")), after)
		respond(w, v, e)
	}))
	mux.Handle("GET /api/v1/worlds/{id}/agents/{agent}/relationships", wrap(func(w http.ResponseWriter, r *http.Request, p auth.Principal, game domain.ID) {
		v, e := lore.Relationships(r.Context(), p, game, domain.ID(r.PathValue("agent")))
		respond(w, map[string]any{"relationships": v}, e)
	}))
	mux.Handle("GET /api/v1/worlds/{id}/agents/{agent}/relationships/{relationship}", wrap(func(w http.ResponseWriter, r *http.Request, p auth.Principal, game domain.ID) {
		after, ok := version(w, r)
		if !ok {
			return
		}
		v, e := lore.RelationshipHistory(r.Context(), p, game, domain.ID(r.PathValue("agent")), domain.ID(r.PathValue("relationship")), after)
		respond(w, v, e)
	}))
	mux.Handle("GET /api/v1/worlds/{id}/traditions/{tradition}", wrap(func(w http.ResponseWriter, r *http.Request, p auth.Principal, game domain.ID) {
		cursor := r.URL.Query().Get("cursor")
		if !cursorOK(w, cursor) {
			return
		}
		after, ok := version(w, r)
		if !ok {
			return
		}
		v, e := lore.TraditionDetail(r.Context(), p, game, domain.ID(r.PathValue("tradition")), cursor, after)
		respond(w, v, e)
	}))
}
func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
