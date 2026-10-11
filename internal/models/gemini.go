// Package models contains the worker's bounded Google GenAI adapter.
package models

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	DefaultTextModel      = "gemma-4-26b-a4b-it"
	DefaultEmbeddingModel = "gemini-embedding-001"
	EmbeddingDimensions   = 768
	maxModelOutputBytes   = 24 * 1024
)

type Gemini struct {
	client            *genai.Client
	model             string
	slots             chan struct{}
	schedule          *scheduler
	embeddingSchedule *scheduler
}

func NewGemini(ctx context.Context, apiKey string) (*Gemini, error) {
	return NewGeminiWithBudget(ctx, apiKey, DefaultBudget())
}

func NewGeminiWithBudget(ctx context.Context, apiKey string, budget Budget) (*Gemini, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required for the worker")
	}
	attempts := int32(1)
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey, Backend: genai.BackendGeminiAPI,
		HTTPClient:  &http.Client{Transport: rateLimitTransport{base: http.DefaultTransport}},
		HTTPOptions: genai.HTTPOptions{RetryOptions: &genai.HTTPRetryOptions{Attempts: &attempts}},
	})
	if err != nil {
		return nil, err
	}
	schedule, err := sharedSchedule(apiKey+"/"+DefaultTextModel, budget.RequestsPerMinute, budget.InputTokensPerMinute, budget.WorkerCount)
	if err != nil {
		return nil, err
	}
	embeddingSchedule, err := sharedSchedule(apiKey+"/"+DefaultEmbeddingModel, budget.EmbeddingRequestsPerMinute, budget.EmbeddingInputTokensPerMinute, budget.WorkerCount)
	if err != nil {
		return nil, err
	}
	return &Gemini{client: client, model: DefaultTextModel, slots: make(chan struct{}, 4), schedule: schedule, embeddingSchedule: embeddingSchedule}, nil
}

func (g *Gemini) GenerateJSON(ctx context.Context, instruction string, image []byte, mimeType string, output any) error {
	if err := g.acquire(ctx); err != nil {
		return err
	}
	defer g.release()
	parts := []*genai.Part{{Text: instruction}}
	if len(image) != 0 {
		if mimeType == "" {
			return fmt.Errorf("image MIME type is required")
		}
		parts = append(parts, &genai.Part{InlineData: &genai.Blob{Data: image, MIMEType: mimeType}})
	}
	response, err := g.generate(ctx, parts)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(response.Text())
	if len(text) == 0 || len(text) > maxModelOutputBytes {
		return fmt.Errorf("model returned empty or oversized JSON output")
	}
	if err = json.Unmarshal([]byte(text), output); err != nil {
		value := reflect.ValueOf(output)
		if value.Kind() != reflect.Pointer || value.IsNil() {
			return fmt.Errorf("model output target must be a pointer")
		}
		value.Elem().Set(reflect.Zero(value.Elem().Type()))
		repair := fmt.Sprintf("%s\nThe prior draft was not valid for the required JSON shape: %s. Repair it and return only valid JSON, no markdown. Draft: %s", instruction, err.Error(), text)
		repairParts := []*genai.Part{{Text: repair + "\nPreserve the original evidence and uncertainty. Restructure the draft; do not invent new observations."}}
		if len(image) != 0 {
			repairParts = append(repairParts, &genai.Part{InlineData: &genai.Blob{Data: image, MIMEType: mimeType}})
		}
		response, repairErr := g.generate(ctx, repairParts)
		if repairErr != nil {
			return repairErr
		}
		text = strings.TrimSpace(response.Text())
		if len(text) == 0 || len(text) > maxModelOutputBytes {
			return fmt.Errorf("model returned empty or oversized repaired JSON")
		}
		if err = json.Unmarshal([]byte(text), output); err != nil {
			return fmt.Errorf("decode repaired model JSON: %w", err)
		}
	}
	return nil
}

func jsonConfig() *genai.GenerateContentConfig {
	return &genai.GenerateContentConfig{ResponseMIMEType: "application/json", MaxOutputTokens: 4096}
}

func (g *Gemini) Embed(ctx context.Context, text string, query bool) ([]float32, error) {
	started := time.Now()
	if err := g.acquire(ctx); err != nil {
		return nil, err
	}
	defer g.release()
	estimate := (len([]rune(text)) + 2) / 3
	waited, err := waitSchedule(ctx, g.embeddingSchedule, estimate)
	if err != nil {
		return nil, err
	}
	taskType := "RETRIEVAL_DOCUMENT"
	if query {
		taskType = "RETRIEVAL_QUERY"
	}
	dimensions := int32(EmbeddingDimensions)
	response, err := g.client.Models.EmbedContent(ctx, DefaultEmbeddingModel, []*genai.Content{{Parts: []*genai.Part{{Text: text}}}}, &genai.EmbedContentConfig{TaskType: taskType, OutputDimensionality: &dimensions})
	trace, _ := ctx.Value(traceKey{}).(Trace)
	slog.InfoContext(ctx, "model_embedding", "game_id", trace.GameID, "round_id", trace.RoundID, "agent_id", trace.AgentID, "phase", trace.Phase, "model", DefaultEmbeddingModel, "estimated_input_tokens", estimate, "queue_wait_ms", waited.Milliseconds(), "duration_ms", time.Since(started).Milliseconds(), "success", err == nil)
	if err != nil {
		return nil, err
	}
	if response == nil || len(response.Embeddings) != 1 || len(response.Embeddings[0].Values) != EmbeddingDimensions {
		return nil, fmt.Errorf("embedding provider returned an invalid vector")
	}
	return response.Embeddings[0].Values, nil
}

func (g *Gemini) acquire(ctx context.Context) error {
	select {
	case g.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *Gemini) release() { <-g.slots }
