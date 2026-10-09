package handlers

import (
	"github.com/amh1k/mythborn/internal/api"
	"github.com/amh1k/mythborn/internal/app"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
	"net/http"
)

func registerLoreRoutes(mux *http.ServeMux, deps api.Dependencies) {
	lore := app.Lore{Store: repository.New(deps.DB)}
	wrap := func(fn http.HandlerFunc) http.Handler { return auth.Middleware(deps.Auth, fn) }
	mux.Handle("GET /api/v1/worlds/{id}/agents", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		agents, err := lore.Agents(r.Context(), p, domain.ID(r.PathValue("id")))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
	}))
	mux.Handle("GET /api/v1/worlds/{id}/agents/{agent}/beliefs", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		beliefs, revisions, err := lore.Beliefs(r.Context(), p, domain.ID(r.PathValue("id")), domain.ID(r.PathValue("agent")))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"beliefs": beliefs, "revisions": revisions})
	}))
	mux.Handle("GET /api/v1/worlds/{id}/traditions", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		traditions, err := lore.Traditions(r.Context(), p, domain.ID(r.PathValue("id")))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"traditions": traditions})
	}))
	mux.Handle("GET /api/v1/worlds/{id}/history", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		history, err := lore.History(r.Context(), p, domain.ID(r.PathValue("id")))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"history": history})
	}))
}
