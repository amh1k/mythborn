package models

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRateLimitTransportHonorsProviderDelay(t *testing.T) {
	for _, test := range []struct {
		name, header, body string
		want               time.Duration
	}{
		{"default", "", "quota exhausted", time.Minute},
		{"seconds header", "90", "", 90 * time.Second},
		{"retry info", "", `{"error":{"details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"120s"}]}}`, 2 * time.Minute},
		{"longest delay", "180", `{"error":{"details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"90s"}]}}`, 3 * time.Minute},
		{"short delay", "5", "", time.Minute},
		{"invalid delay", "garbage", `{"error":{"details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"bad"}]}}`, time.Minute},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := rateLimitTransport{base: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{test.header}}, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})}
			_, err := transport.RoundTrip(&http.Request{})
			if delay, limited := RateLimitDelay(fmt.Errorf("wrapped: %w", err)); !limited || delay != test.want {
				t.Fatalf("want %v, got %v (limited=%v)", test.want, delay, limited)
			}
			if strings.Contains(err.Error(), "quota exhausted") {
				t.Fatal("provider error body leaked into scheduling error")
			}
		})
	}
	until := time.Now().Add(3 * time.Minute).UTC().Format(http.TimeFormat)
	transport := rateLimitTransport{base: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{until}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	_, err := transport.RoundTrip(&http.Request{})
	if delay, _ := RateLimitDelay(err); delay < 179*time.Second || delay > 180*time.Second {
		t.Fatalf("HTTP date delay was lost: %v", delay)
	}
	if _, limited := RateLimitDelay(genai.APIError{Code: 503}); limited {
		t.Fatal("ordinary failure classified as a rate limit")
	}
}

func TestGeminiRateLimitsReachWorkflowWithoutSDKRetries(t *testing.T) {
	for _, kind := range []string{"generation", "repair", "embedding"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			attempts := int32(1)
			client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
				APIKey: "test-key", Backend: genai.BackendGeminiAPI,
				HTTPOptions: genai.HTTPOptions{RetryOptions: &genai.HTTPRetryOptions{Attempts: &attempts}},
				HTTPClient: &http.Client{Transport: rateLimitTransport{base: roundTripFunc(func(*http.Request) (*http.Response, error) {
					calls++
					status, body := 429, `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED"}}`
					if kind == "repair" && calls == 1 {
						status, body = 200, `{"candidates":[{"content":{"parts":[{"text":"broken JSON"}]}}]}`
					}
					return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
				})}},
			})
			if err != nil {
				t.Fatal(err)
			}
			model := &Gemini{client: client, model: DefaultTextModel, slots: make(chan struct{}, 4)}
			if kind == "embedding" {
				_, err = model.Embed(context.Background(), "test", true)
			} else {
				var output map[string]any
				err = model.GenerateJSON(context.Background(), "test", nil, "", &output)
			}
			wantCalls := 1
			if kind == "repair" {
				wantCalls = 2
			}
			if delay, limited := RateLimitDelay(err); !limited || delay != time.Minute || calls != wantCalls || len(model.slots) != 0 {
				t.Fatalf("rate limit not returned promptly: delay=%v, limited=%v, calls=%d, slots=%d, err=%v", delay, limited, calls, len(model.slots), err)
			}
		})
	}
}
