package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amh1k/mythborn/internal/api"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
)

// Only round/game reads are implemented. Any attempt to write or read a
// Messenger context for these roles fails the test immediately.
type debateAuthDB struct {
	contracts.Database
	role domain.PlayerRole
}
type debateRow func(...any) error

func (r debateRow) Scan(dest ...any) error { return r(dest...) }
func (db debateAuthDB) QueryRow(_ context.Context, query string, _ ...any) contracts.Row {
	return debateRow(func(dest ...any) error {
		switch {
		case strings.Contains(query, "FROM rounds"):
			*dest[0].(*string) = "round"
			*dest[1].(*string) = "game"
			*dest[3].(*string) = "discovery"
			*dest[5].(*string) = "running"
			*dest[6].(*string) = "reacting"
		case strings.Contains(query, "FROM games"):
			*dest[0].(*string) = "game"
			*dest[1].(*string) = "owner"
			*dest[3].(*string) = string(db.role)
			*dest[4].(*string) = "active"
		default:
			panic("unexpected database access")
		}
		return nil
	})
}

type debateAuth struct{}

func (debateAuth) Authenticate(context.Context, string) (auth.Principal, error) {
	return auth.Principal{AccountID: "owner"}, nil
}

type liveDebateReader struct{}

func (liveDebateReader) Read(context.Context, domain.ID, domain.ID) (*workflow.DebatePreview, error) {
	return &workflow.DebatePreview{GameID: "game", RoundID: "round"}, nil
}

func TestWatchingDoesNotGrantMessagingPermissions(t *testing.T) {
	for _, role := range []domain.PlayerRole{domain.PlayerRoleObserver, domain.PlayerRoleGod} {
		t.Run(string(role), func(t *testing.T) {
			handler := api.NewHandler(api.Dependencies{DB: debateAuthDB{role: role}, Auth: debateAuth{}, Debates: liveDebateReader{}}, Registrar{})
			for _, test := range []struct {
				method, path string
				status       int
			}{
				{http.MethodGet, "/api/v1/episodes/round/debate", http.StatusOK},
				{http.MethodPost, "/api/v1/episodes/round/debate", http.StatusMethodNotAllowed},
				{http.MethodGet, "/api/v1/episodes/round/agents/agent/messages", http.StatusForbidden},
				{http.MethodPost, "/api/v1/episodes/round/agents/agent/messages", http.StatusForbidden},
			} {
				req := httptest.NewRequest(test.method, test.path, strings.NewReader(`{"message":"Please listen to me"}`))
				req.Header.Set("Authorization", "Bearer preview")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				if response.Code != test.status {
					t.Errorf("%s %s = %d, want %d: %s", test.method, test.path, response.Code, test.status, response.Body.String())
				}
				if test.path == "/api/v1/episodes/round/debate" && test.method == http.MethodGet && response.Header().Get("Cache-Control") != "no-store" {
					t.Error("live preview may be cached")
				}
			}
		})
	}
}
