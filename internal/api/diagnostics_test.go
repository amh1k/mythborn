package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorsHaveDistinctTraceableRequestReferences(t *testing.T) {
	handler := NewHandler(Dependencies{}, RouteRegistrarFunc(func(mux *http.ServeMux, _ Dependencies) {
		mux.HandleFunc("GET /failure", func(w http.ResponseWriter, r *http.Request) {
			writeAPIError(w, http.StatusServiceUnavailable, "not_ready")
		})
	}))
	previous := ""
	for range 2 {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/failure", nil))
		var body map[string]string
		_ = json.Unmarshal(response.Body.Bytes(), &body)
		id := response.Header().Get("X-Request-ID")
		if len(id) != 32 || body["request_id"] != id || id == previous {
			t.Fatalf("invalid error reference: %v", body)
		}
		previous = id
	}
}
