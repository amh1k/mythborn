package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amh1k/mythborn/internal/app"
	"github.com/amh1k/mythborn/internal/repository"
)

func TestWriteErrorMissingTemplatesExplainsRequiredSetup(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeError(recorder, fmt.Errorf("%w: %w", app.ErrConflict, repository.ErrMissingTemplates))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["code"] != "agent_templates_not_configured" {
		t.Fatalf("unexpected error code: %q", payload["code"])
	}
	for _, agent := range []string{"priest", "scientist", "soldier", "historian"} {
		if !strings.Contains(payload["message"], agent) {
			t.Errorf("message does not identify required %s template", agent)
		}
	}
}

func TestWriteErrorDoesNotExposeInternalDetails(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeError(recorder, errors.New("database connection failed: sensitive connection details"))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "sensitive") {
		t.Fatal("response exposed internal error details")
	}
}
