package models

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/genai"
)

const MaxPromptBytes = 48 * 1024

type Budget struct{ RequestsPerMinute, InputTokensPerMinute, EmbeddingRequestsPerMinute, EmbeddingInputTokensPerMinute, WorkerCount int }

func DefaultBudget() Budget {
	return Budget{RequestsPerMinute: 10, InputTokensPerMinute: 60000, EmbeddingRequestsPerMinute: 15, EmbeddingInputTokensPerMinute: 100000, WorkerCount: 1}
}

type scheduler struct {
	calls  *rate.Limiter
	tokens *tokenWindow
	gate   chan struct{}
}

type tokenReservation struct {
	at     time.Time
	tokens int
}
type tokenWindow struct {
	sync.Mutex
	limit        int
	reservations []tokenReservation
}

func (w *tokenWindow) Burst() int { return w.limit }
func (w *tokenWindow) WaitN(ctx context.Context, tokens int) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		w.Lock()
		now := time.Now()
		used := 0
		first := 0
		for first < len(w.reservations) && !w.reservations[first].at.Add(time.Minute).After(now) {
			first++
		}
		w.reservations = append([]tokenReservation(nil), w.reservations[first:]...)
		for _, r := range w.reservations {
			used += r.tokens
		}
		if used+tokens <= w.limit {
			w.reservations = append(w.reservations, tokenReservation{now, tokens})
			w.Unlock()
			return nil
		}
		delay := time.Until(w.reservations[0].at.Add(time.Minute))
		w.Unlock()
		timer := time.NewTimer(max(delay, time.Millisecond))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

var schedules = struct {
	sync.Mutex
	values map[[32]byte]*scheduler
}{values: map[[32]byte]*scheduler{}}

func sharedSchedule(key string, rpmTotal, tpmTotal, workers int) (*scheduler, error) {
	if workers < 1 || rpmTotal < workers || tpmTotal < 1024*workers {
		return nil, fmt.Errorf("model budgets must provide at least one request and 1024 input tokens per worker per minute")
	}
	// Instances using one provider key share spacing in this process. Across
	// processes, each worker must use the same declared total worker count.
	id := sha256.Sum256([]byte(fmt.Sprintf("%s/%d/%d/%d", key, rpmTotal, tpmTotal, workers)))
	schedules.Lock()
	defer schedules.Unlock()
	if cached := schedules.values[id]; cached != nil {
		return cached, nil
	}
	rpm, tpm := rpmTotal/workers, tpmTotal/workers
	result := &scheduler{calls: rate.NewLimiter(rate.Limit(float64(rpm)/60), 1), tokens: &tokenWindow{limit: tpm}, gate: make(chan struct{}, 1)}
	schedules.values[id] = result
	return result, nil
}

func waitSchedule(ctx context.Context, schedule *scheduler, estimate int) (time.Duration, error) {
	if schedule == nil {
		return 0, nil
	}
	if estimate > schedule.tokens.Burst() {
		return 0, fmt.Errorf("prompt exceeds the configured input token allowance per worker")
	}
	started := time.Now()
	select {
	case schedule.gate <- struct{}{}:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	err := schedule.tokens.WaitN(ctx, estimate)
	if err == nil {
		err = schedule.calls.Wait(ctx)
	}
	<-schedule.gate
	return time.Since(started), err
}

type traceKey struct{}
type Trace struct{ GameID, RoundID, AgentID, Phase string }

func WithTrace(ctx context.Context, trace Trace) context.Context {
	return context.WithValue(ctx, traceKey{}, trace)
}

func (g *Gemini) generate(ctx context.Context, parts []*genai.Part) (response *genai.GenerateContentResponse, err error) {
	started := time.Now()
	bytes, estimate := 0, 0
	for _, part := range parts {
		bytes += len(part.Text)
		estimate += (len([]rune(part.Text)) + 2) / 3
		if part.InlineData != nil {
			estimate += 4096
		}
	}
	if bytes > MaxPromptBytes {
		return nil, fmt.Errorf("model prompt exceeds the %d byte budget", MaxPromptBytes)
	}
	waited, err := waitSchedule(ctx, g.schedule, estimate)
	if err != nil {
		return nil, err
	}

	trace, _ := ctx.Value(traceKey{}).(Trace)
	response, err = g.client.Models.GenerateContent(ctx, g.model, []*genai.Content{{Parts: parts}}, jsonConfigForPhase(trace.Phase))
	attributes := []any{"game_id", trace.GameID, "round_id", trace.RoundID, "agent_id", trace.AgentID, "phase", trace.Phase, "model", g.model, "prompt_bytes", bytes, "estimated_input_tokens", estimate, "queue_wait_ms", waited.Milliseconds(), "duration_ms", time.Since(started).Milliseconds(), "success", err == nil}
	if response != nil && response.UsageMetadata != nil {
		attributes = append(attributes, "input_tokens", response.UsageMetadata.PromptTokenCount, "output_tokens", response.UsageMetadata.CandidatesTokenCount, "thinking_tokens", response.UsageMetadata.ThoughtsTokenCount)
	}
	if response != nil && len(response.Candidates) > 0 {
		attributes = append(attributes, "finish_reason", response.Candidates[0].FinishReason)
	}
	slog.InfoContext(ctx, "model_generation", attributes...)
	return response, err
}
