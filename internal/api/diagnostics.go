package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type measuredResponse struct {
	http.ResponseWriter
	status   int
	response bytes.Buffer
}

func (w *measuredResponse) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *measuredResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.response.Len()+len(data) <= 16384 {
		_, _ = w.response.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func diagnostics(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bytes [16]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "request_reference_unavailable")
			return
		}
		id := hex.EncodeToString(bytes[:])
		w.Header().Set("X-Request-ID", id)
		started := time.Now()
		measured := &measuredResponse{ResponseWriter: w}
		next.ServeHTTP(measured, r)
		if measured.status == 0 {
			measured.status = http.StatusOK
		}
		fields := []any{"request_id", id, "method", r.Method, "path", r.URL.Path, "status", measured.status, "duration_ms", time.Since(started).Milliseconds()}
		// Only identifiers leave the response; never log player messages, photos,
		// model prompts, credentials, or the complete response body.
		var body struct {
			Data      json.RawMessage `json:"data"`
			CommandID string          `json:"command_id"`
			EpisodeID string          `json:"episode_id"`
		}
		if json.Unmarshal(measured.response.Bytes(), &body) == nil {
			if len(body.Data) > 0 {
				_ = json.Unmarshal(body.Data, &body)
			}
			if body.CommandID != "" {
				fields = append(fields, "command_id", body.CommandID)
			}
			if body.EpisodeID != "" {
				fields = append(fields, "round_id", body.EpisodeID)
			}
		}
		logger.Info("api_request", fields...)
	})
}
