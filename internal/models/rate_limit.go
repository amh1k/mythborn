package models

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/genai"
)

const defaultRateLimitDelay = time.Minute

// RateLimitError contains scheduling information, never a character response.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string { return "model rate limit reached" }

func RateLimitDelay(err error) (time.Duration, bool) {
	var limited *RateLimitError
	if errors.As(err, &limited) {
		return max(defaultRateLimitDelay, limited.RetryAfter), true
	}
	var apiError genai.APIError
	if errors.As(err, &apiError) && apiError.Code == http.StatusTooManyRequests {
		return retryInfoDelay(apiError.Details), true
	}
	return 0, false
}

func retryInfoDelay(details []map[string]any) time.Duration {
	delay := defaultRateLimitDelay
	for _, detail := range details {
		if detail["@type"] != "type.googleapis.com/google.rpc.RetryInfo" {
			continue
		}
		if value, ok := detail["retryDelay"].(string); ok {
			if parsed, err := time.ParseDuration(value); err == nil {
				delay = max(delay, parsed)
			}
		}
	}
	return delay
}

// The SDK does not expose error response headers. Capture Retry-After here,
// before it discards the response, so the durable workflow owns the wait.
type rateLimitTransport struct{ base http.RoundTripper }

func (t rateLimitTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err != nil || response.StatusCode != http.StatusTooManyRequests {
		return response, err
	}
	defer response.Body.Close()
	var body struct {
		Error genai.APIError `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&body)
	delay := retryInfoDelay(body.Error.Details)
	header := response.Header.Get("Retry-After")
	if seconds, parseErr := strconv.ParseFloat(header, 64); parseErr == nil && seconds > 0 && seconds < float64((1<<63-1)/int64(time.Second)) {
		delay = max(delay, time.Duration(seconds*float64(time.Second)))
	} else if until, parseErr := http.ParseTime(header); parseErr == nil {
		delay = max(delay, time.Until(until))
	}
	return nil, &RateLimitError{RetryAfter: delay}
}
