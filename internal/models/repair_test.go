package models

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"
)

type requestTransport func(*http.Request) (*http.Response, error)

func (f requestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRepairKeepsImageAndDoesNotInventObservations(t *testing.T) {
	calls := 0
	transport := requestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		var input map[string]any
		_ = json.NewDecoder(r.Body).Decode(&input)
		contents := input["contents"].([]any)
		parts := contents[0].(map[string]any)["parts"].([]any)
		if len(parts) != 2 {
			t.Errorf("request %d lost image evidence", calls)
		}
		if calls == 2 && !strings.Contains(parts[0].(map[string]any)["text"].(string), "do not invent") {
			t.Error("repair lacks evidence constraint")
		}
		text := "{invalid"
		if calls == 2 {
			text = `{"description":"A stone"}`
		}
		response, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]string{"text": text}}}}}})
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(response)))}, nil
	})
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: "test-key", Backend: genai.BackendGeminiAPI, HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	model := Gemini{client: client, model: "test-model", slots: make(chan struct{}, 4)}
	var output struct {
		Description string `json:"description"`
	}
	if err := model.GenerateJSON(context.Background(), "Describe visible evidence", []byte("photo"), "image/jpeg", &output); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || output.Description != "A stone" {
		t.Fatalf("repair did not complete: %d %+v", calls, output)
	}
}

func TestSharedBudgetSpacesWorldsAndHonorsCancellation(t *testing.T) {
	first, err := sharedSchedule("budget-test-key", 60, 6000, 2)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := sharedSchedule("budget-test-key", 60, 6000, 2)
	if first != second {
		t.Fatal("clients do not share provider scheduling")
	}
	if err := first.calls.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := second.calls.Wait(ctx); err == nil {
		t.Fatal("a second world sent an immediate burst")
	}
	if _, err := sharedSchedule("bad", 1, 6000, 2); err == nil {
		t.Fatal("invalid per-worker allocation accepted")
	}
}

func TestInputTokenBudgetUsesRollingMinute(t *testing.T) {
	window := &tokenWindow{limit: 100}
	window.reservations = append(window.reservations, tokenReservation{at: time.Now(), tokens: 80})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := window.WaitN(ctx, 21); err == nil {
		t.Fatal("rolling input-token allowance was exceeded")
	}
	window.Lock()
	window.reservations[0].at = time.Now().Add(-61 * time.Second)
	window.Unlock()
	if err := window.WaitN(context.Background(), 100); err != nil {
		t.Fatalf("expired token reservation was not released: %v", err)
	}
}
