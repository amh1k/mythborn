package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amh1k/mythborn/internal/api"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/domain"
)

type roundListDB struct {
	debateAuthDB
	lists int
}

func (db *roundListDB) Query(_ context.Context, query string, args ...any) (contracts.Rows, error) {
	if !strings.Contains(query, "FROM rounds WHERE game_id=$1 ORDER BY sequence_number DESC LIMIT 50") || len(args) != 1 || args[0] != "game" {
		panic("unexpected or unscoped round listing")
	}
	db.lists++
	return &roundListRows{}, nil
}

type roundListRows struct{ index int }

func (r *roundListRows) Next() bool { r.index++; return r.index <= 2 }
func (r *roundListRows) Err() error { return nil }
func (r *roundListRows) Close()     {}
func (r *roundListRows) Scan(dest ...any) error {
	*dest[0].(*string), *dest[1].(*string), *dest[3].(*string) = "round", "game", "discovery"
	*dest[2].(*int64) = int64(3 - r.index)
	if r.index == 1 {
		*dest[5].(*string), *dest[6].(*string) = "running", "debating"
	} else {
		*dest[5].(*string), *dest[6].(*string) = "complete", "complete"
	}
	return nil
}

type roundListAuth struct{ principal auth.Principal }

func (a roundListAuth) Authenticate(context.Context, string) (auth.Principal, error) {
	return a.principal, nil
}

func TestRoundListIncludesUnfinishedProgressAndChecksOwnership(t *testing.T) {
	for _, test := range []struct {
		name      string
		principal auth.Principal
		status    int
	}{
		{"observer owner", auth.Principal{AccountID: "owner"}, http.StatusOK},
		{"admin", auth.Principal{AccountID: "admin", SystemRole: domain.SystemRoleAdmin}, http.StatusOK},
		{"different owner", auth.Principal{AccountID: "other"}, http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := &roundListDB{debateAuthDB: debateAuthDB{role: domain.PlayerRoleObserver}}
			handler := api.NewHandler(api.Dependencies{DB: db, Auth: roundListAuth{test.principal}}, Registrar{})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/worlds/game/episodes", nil)
			req.Header.Set("Authorization", "Bearer preview")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.status, response.Body.String())
			}
			if test.status == http.StatusForbidden {
				if db.lists != 0 {
					t.Fatal("listed rounds before authorizing owner")
				}
				return
			}
			var result struct {
				Episodes []domain.Round `json:"episodes"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Episodes) != 2 || result.Episodes[0].Stage != domain.RoundStageDebating || result.Episodes[0].Status != domain.RoundStatusRunning {
				t.Fatalf("unfinished round progress missing: %+v", result)
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("progress may be cached")
			}
		})
	}
}
