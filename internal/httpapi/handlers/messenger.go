package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/amh1k/mythborn/internal/api"
	"github.com/amh1k/mythborn/internal/app"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
	"github.com/amh1k/mythborn/internal/workflow"
)

func registerMessengerRoutes(mux *http.ServeMux, deps api.Dependencies) {
	store := repository.New(deps.DB)
	messenger := app.Messenger{Store: store}
	commands := app.Commands{Store: store}
	wrap := func(fn http.HandlerFunc) http.Handler { return auth.Middleware(deps.Auth, fn) }
	pattern := "/api/v1/episodes/{id}/agents/{agent}/messages"
	mux.Handle("GET "+pattern, wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		thread, err := messenger.Thread(r.Context(), p, domain.ID(r.PathValue("id")), domain.ID(r.PathValue("agent")))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"summary": thread.Summary, "summary_version": func() int64 {
			if thread.Summary == nil {
				return 0
			}
			return thread.Summary.Version
		}()})
	}))
	mux.Handle("POST "+pattern, wrap(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Message                string `json:"message"`
			ExpectedSummaryVersion int64  `json:"expected_summary_version"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req) != nil {
			writeError(w, app.ErrInvalid)
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		id, err := messenger.Send(r.Context(), p, domain.ID(r.PathValue("id")), domain.ID(r.PathValue("agent")), req.Message, idempotencyKey(r), req.ExpectedSummaryVersion)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]domain.ID{"command_id": id})
	}))
	mux.Handle("GET /api/v1/commands/{id}", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		command, err := commands.Get(r.Context(), p, domain.ID(r.PathValue("id")))
		if err != nil {
			writeError(w, err)
			return
		}
		response := map[string]any{"command": map[string]any{"id": command.ID, "status": command.Status}}
		if command.GameID != nil && command.RoundID != nil && len(command.Payload) > 0 {
			var payload workflow.CommandPayload
			if json.Unmarshal(command.Payload, &payload) == nil && payload.AgentID != "" {
				thread, e := messenger.Thread(r.Context(), p, *command.RoundID, payload.AgentID)
				if e == nil && thread.Summary != nil {
					response["summary"] = thread.Summary
				}
			}
		}
		if len(command.ResultPayload) > 0 {
			response["reply"] = json.RawMessage(command.ResultPayload)
		}
		writeJSON(w, http.StatusOK, response)
	}))
}
