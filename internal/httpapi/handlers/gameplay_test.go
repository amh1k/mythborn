package handlers

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/amh1k/mythborn/internal/api"
	"github.com/amh1k/mythborn/internal/contracts"
)

type inaccessibleGameplayDB struct{ contracts.Database }

func (inaccessibleGameplayDB) QueryRow(context.Context, string, ...any) contracts.Row {
	return debateRow(func(...any) error { return contracts.ErrNoRows })
}
func TestGameplayReadsRequireOwnershipAndRejectMalformedPagination(t *testing.T) {
	h := api.NewHandler(api.Dependencies{DB: inaccessibleGameplayDB{}, Auth: debateAuth{}}, Registrar{})
	world := "00000000-0000-4000-8000-000000000001"
	agent := "00000000-0000-4000-8000-000000000002"
	for _, suffix := range []string{"/mysteries", "/mysteries/" + agent, "/agents/" + agent + "/relationships", "/agents/" + agent + "/relationships/" + agent, "/traditions/" + agent} {
		req := httptest.NewRequest("GET", "/api/v1/worlds/"+world+suffix, nil)
		req.Header.Set("Authorization", "Bearer test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 404 {
			t.Fatalf("unowned %s status %d: %s", suffix, w.Code, w.Body.String())
		}
		req = httptest.NewRequest("GET", "/api/v1/worlds/"+world+suffix, nil)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("anonymous %s status %d", suffix, w.Code)
		}
	}
	for _, suffix := range []string{"/mysteries?cursor=invalid", "/mysteries?state=unknown", "/mysteries/" + agent + "?after_version=-1", "/traditions/" + agent + "?after_version=NaN", "/agents/invalid/relationships"} {
		req := httptest.NewRequest("GET", "/api/v1/worlds/"+world+suffix, nil)
		req.Header.Set("Authorization", "Bearer test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 422 {
			t.Fatalf("invalid %s status %d", suffix, w.Code)
		}
	}
	req := httptest.NewRequest("POST", "/api/v1/worlds/"+world+"/mysteries", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 405 {
		t.Fatal("player can mutate mystery ledger")
	}
}
