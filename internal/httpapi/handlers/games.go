package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/amh1k/mythborn/internal/api"
	"github.com/amh1k/mythborn/internal/app"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
)

type Registrar struct{}

func (Registrar) RegisterRoutes(mux *http.ServeMux, deps api.Dependencies) {
	registerEpisodeRoutes(mux, deps)
	registerMessengerRoutes(mux, deps)
	registerLoreRoutes(mux, deps)
	registerAdminRoutes(mux, deps)
	registerAccountRoutes(mux, deps)
	service := app.Games{Store: repository.New(deps.DB)}
	wrap := func(fn http.HandlerFunc) http.Handler { return auth.Middleware(deps.Auth, fn) }
	mux.Handle("GET /api/v1/worlds", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		worlds, err := service.List(r.Context(), p)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"worlds": worlds})
	}))
	mux.Handle("POST /api/v1/worlds", wrap(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name       string            `json:"name"`
			PlayerRole domain.PlayerRole `json:"player_role"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req) != nil {
			writeError(w, app.ErrInvalid)
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		game, commandID, err := service.Create(r.Context(), p, req.Name, req.PlayerRole, idempotencyKey(r))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"world": game, "command_id": commandID})
	}))
	mux.Handle("GET /api/v1/worlds/{id}", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		game, err := service.Get(r.Context(), p, domain.ID(r.PathValue("id")))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"world": game})
	}))
	mux.Handle("PATCH /api/v1/worlds/{id}/settings", wrap(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ReviewPhotoDescription *bool `json:"review_photo_description"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req) != nil || req.ReviewPhotoDescription == nil {
			writeError(w, app.ErrInvalid)
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		if err := service.SetReview(r.Context(), p, domain.ID(r.PathValue("id")), *req.ReviewPhotoDescription); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"review_photo_description": *req.ReviewPhotoDescription})
	}))
	mux.Handle("POST /api/v1/worlds/{id}/end", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		id, err := service.End(r.Context(), p, domain.ID(r.PathValue("id")), idempotencyKey(r))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]domain.ID{"command_id": id})
	}))
	mux.Handle("DELETE /api/v1/worlds/{id}", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		commandID, err := service.Delete(r.Context(), p, domain.ID(r.PathValue("id")))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]domain.ID{"command_id": commandID})
	}))
}

func idempotencyKey(r *http.Request) string {
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		return key
	}
	return r.URL.Query().Get("idempotency_key")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, app.ErrInvalid):
		status, code = http.StatusUnprocessableEntity, "invalid_request"
	case errors.Is(err, app.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, app.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	case errors.Is(err, repository.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, repository.ErrStateConflict), errors.Is(err, repository.ErrDuplicateObservation):
		status, code = http.StatusConflict, "conflict"
	}
	writeJSON(w, status, map[string]string{"error": code, "code": code})
}
