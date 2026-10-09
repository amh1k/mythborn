package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/amh1k/mythborn/internal/api"
	"github.com/amh1k/mythborn/internal/app"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
)

func registerEpisodeRoutes(mux *http.ServeMux, deps api.Dependencies) {
	store := repository.New(deps.DB)
	episodes := app.Episodes{Store: store}
	observations := app.Observations{Store: store, Photos: deps.Photos}
	wrap := func(fn http.HandlerFunc) http.Handler { return auth.Middleware(deps.Auth, fn) }
	mux.Handle("POST /api/v1/worlds/{id}/observations", wrap(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
		if err := r.ParseMultipartForm(11 << 20); err != nil {
			writeError(w, app.ErrInvalid)
			return
		}
		file, header, err := r.FormFile("photo")
		if err != nil {
			writeError(w, app.ErrInvalid)
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
		if err != nil || len(data) > 10<<20 {
			writeError(w, app.ErrInvalid)
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		obsID, episodeID, commandID, err := observations.Create(r.Context(), p, domain.ID(r.PathValue("id")), header.Filename, data, r.FormValue("player_statement"), idempotencyKey(r), r.FormValue("allow_reuse") == "true")
		if errors.Is(err, repository.ErrDuplicateObservation) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "duplicate_observation", "code": "duplicate_observation", "episode_id": episodeID})
			return
		}
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]domain.ID{"observation_id": obsID, "episode_id": episodeID, "command_id": commandID})
	}))
	mux.Handle("GET /api/v1/episodes/{id}", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		detail, err := episodes.Get(r.Context(), p, domain.ID(r.PathValue("id")))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}))
	mux.Handle("POST /api/v1/episodes/{id}/description-decision", wrap(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Decision      string `json:"decision"`
			Correction    string `json:"player_correction"`
			ClarityChoice string `json:"clarity_choice"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req) != nil {
			writeError(w, app.ErrInvalid)
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		id, err := episodes.DecideDescription(r.Context(), p, domain.ID(r.PathValue("id")), req.Decision, req.Correction, req.ClarityChoice, idempotencyKey(r))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]domain.ID{"command_id": id})
	}))
	mux.Handle("POST /api/v1/episodes/{id}/retry", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		id, err := episodes.Retry(r.Context(), p, domain.ID(r.PathValue("id")), idempotencyKey(r))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]domain.ID{"command_id": id})
	}))
	mux.Handle("GET /api/v1/observations/{id}/photo-url", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		observation, err := store.GetObservationByID(r.Context(), domain.ID(r.PathValue("id")))
		if err != nil {
			writeError(w, err)
			return
		}
		if _, err = appGameAccess(r, p, store, observation.GameID); err != nil {
			writeError(w, err)
			return
		}
		if deps.Photos == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "photo_storage_unavailable", "code": "photo_storage_unavailable"})
			return
		}
		url, err := deps.Photos.SignedURL(r.Context(), observation.PhotoObjectPath, 10*time.Minute)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"url": url})
	}))
}

func appGameAccess(r *http.Request, p auth.Principal, store *repository.Store, id domain.ID) (domain.Game, error) {
	game, err := store.GetGame(r.Context(), id)
	if err != nil {
		return game, err
	}
	if game.OwnerAccountID != p.AccountID && !p.IsAdmin() {
		return game, app.ErrForbidden
	}
	return game, nil
}

func (Registrar) registerEpisodeRoutes(mux *http.ServeMux, deps api.Dependencies) {
	registerEpisodeRoutes(mux, deps)
}
