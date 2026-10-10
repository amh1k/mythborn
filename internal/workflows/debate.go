package workflows

import (
	"context"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/converter"
	tw "go.temporal.io/sdk/workflow"
)

type WorkflowQuerier interface {
	QueryWorkflow(context.Context, string, string, string, ...interface{}) (converter.EncodedValue, error)
}

type DebateReader struct{ Client WorkflowQuerier }

func (r DebateReader) Read(ctx context.Context, gameID, roundID domain.ID) (*workflow.DebatePreview, error) {
	value, err := r.Client.QueryWorkflow(ctx, workflow.WorkflowID(gameID), "", workflow.DebatePreviewQueryName, roundID)
	if err != nil {
		return nil, err
	}
	var preview *workflow.DebatePreview
	if err := value.Get(&preview); err != nil {
		return nil, err
	}
	return preview, nil
}

func registerDebateQuery(ctx tw.Context, progress map[domain.ID]*RoundProgress) error {
	return tw.SetQueryHandler(ctx, workflow.DebatePreviewQueryName, func(roundID domain.ID) (*workflow.DebatePreview, error) {
		if round := progress[roundID]; round != nil {
			return round.Preview, nil
		}
		return nil, nil
	})
}

func newDebatePreview(command workflow.Command, current workflow.RoundContext, progress *RoundProgress) *workflow.DebatePreview {
	preview := &workflow.DebatePreview{GameID: command.GameID, RoundID: command.RoundID, AttemptID: command.EventID, Phase: "reaction", Agents: make([]workflow.DebateAgent, 0, len(current.Agents)), Entries: []workflow.DebateEntry{}}
	for _, agent := range current.Agents {
		preview.Agents = append(preview.Agents, workflow.DebateAgent{AgentID: agent.Agent.ID, Type: agent.Agent.Type, DisplayName: agent.Agent.DisplayName})
	}
	for i := range progress.Reactions {
		publishDebateResult(preview, workflow.AgentResult{Reaction: &progress.Reactions[i]})
	}
	for i := range progress.Rebuttals {
		publishDebateResult(preview, workflow.AgentResult{Rebuttal: &progress.Rebuttals[i]})
	}
	return preview
}

func publishDebateResult(preview *workflow.DebatePreview, result workflow.AgentResult) {
	if preview == nil {
		return
	}
	if result.Reaction != nil {
		r := result.Reaction
		preview.Entries = append(preview.Entries, workflow.DebateEntry{ID: r.EventID, AgentID: r.AgentID, Phase: "reaction", Text: r.Interpretation, Reasoning: r.Reasoning})
	}
	if result.Rebuttal != nil {
		r := result.Rebuttal
		preview.Entries = append(preview.Entries, workflow.DebateEntry{ID: r.EventID, AgentID: r.AgentID, Phase: "rebuttal", Text: r.Response})
	}
}
