package handlers

import (
	"encoding/json"
	"github.com/amh1k/mythborn/internal/api"
	"github.com/amh1k/mythborn/internal/app"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
	"net/http"
)

func registerAdminRoutes(mux *http.ServeMux, deps api.Dependencies) {
	admin := app.Admin{Store: repository.New(deps.DB)}
	wrap := func(fn http.HandlerFunc) http.Handler { return auth.Middleware(deps.Auth, fn) }
	mux.Handle("GET /api/v1/admin/templates", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		rows, err := admin.Templates(r.Context(), p)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"templates": rows})
	}))
	mux.Handle("POST /api/v1/admin/templates", wrap(func(w http.ResponseWriter, r *http.Request) {
		var t repository.AgentTemplate
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&t) != nil {
			writeError(w, app.ErrInvalid)
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		created, err := admin.CreateTemplate(r.Context(), p, t)
		if err != nil {
			if err == app.ErrForbidden {
				writeError(w, err)
			} else {
				writeError(w, app.ErrInvalid)
			}
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"template": created})
	}))
	mux.Handle("POST /api/v1/admin/templates/{id}/activate", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		if err := admin.ActivateTemplate(r.Context(), p, domain.ID(r.PathValue("id"))); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.Handle("GET /api/v1/admin/accounts", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		rows, err := admin.Accounts(r.Context(), p)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"accounts": rows})
	}))
	mux.Handle("PATCH /api/v1/admin/accounts/{id}", wrap(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SystemRole domain.SystemRole `json:"system_role"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req) != nil {
			writeError(w, app.ErrInvalid)
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		if err := admin.SetAccountRole(r.Context(), p, domain.ID(r.PathValue("id")), req.SystemRole); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]domain.SystemRole{"system_role": req.SystemRole})
	}))
	mux.Handle("GET /api/v1/admin/games", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		rows, err := admin.Games(r.Context(), p)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"games": rows})
	}))
}
