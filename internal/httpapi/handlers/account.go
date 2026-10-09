package handlers

import (
	"github.com/amh1k/mythborn/internal/api"
	"github.com/amh1k/mythborn/internal/app"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/repository"
	"net/http"
)

func registerAccountRoutes(mux *http.ServeMux, deps api.Dependencies) {
	accounts := app.Accounts{Store: repository.New(deps.DB)}
	mux.Handle("DELETE /api/v1/account", auth.Middleware(deps.Auth, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		job, command, err := accounts.Delete(r.Context(), p)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"deletion_job_id": job, "command_id": command})
	})))
}
