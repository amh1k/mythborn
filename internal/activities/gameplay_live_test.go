package activities

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/gameplay"
	"github.com/amh1k/mythborn/internal/models"
	"github.com/amh1k/mythborn/internal/workflow"
)

// Opt-in provider validation uses fictional committed context and nine generation calls.
// It never writes a game, stores a photo, or consumes credentials in ordinary CI.
func TestLiveGameplayRound(t *testing.T) {
	if os.Getenv("MYTHBORN_LIVE_GAMEPLAY_TEST") != "true" {
		t.Skip("opt in with MYTHBORN_LIVE_GAMEPLAY_TEST=true and GEMINI_API_KEY")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	budget := models.DefaultBudget()
	budget.WorkerCount = 2 // Account for the already-running local worker.
	model, err := models.NewGeminiWithBudget(ctx, os.Getenv("GEMINI_API_KEY"), budget)
	if err != nil {
		t.Fatal(err)
	}
	a := Activities{Models: model}
	ids := []domain.ID{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000003", "00000000-0000-4000-8000-000000000004"}
	kinds := []domain.AgentType{domain.AgentTypePriest, domain.AgentTypeScientist, domain.AgentTypeSoldier, domain.AgentTypeHistorian}
	g := gameplay.Context{ContractVersion: 1, Features: gameplay.Enabled(), OpenMysteryCount: 1, Mysteries: []gameplay.Mystery{{ID: "00000000-0000-4000-8000-000000000010", Question: "Does moss follow moisture?", Title: "Marked stones", State: "open", EvidenceSummary: "The first recorded photo showed moss on a damp stone.", Version: 1, Explanations: []gameplay.Explanation{{Key: "moisture", Claim: "Moss follows damp conditions.", AgentIDs: []domain.ID{ids[1]}}, {Key: "sign", Claim: "The markings may be sacred signs.", AgentIDs: []domain.ID{ids[0]}}}}}}
	world := domain.ID("00000000-0000-4000-8000-000000000020")
	round := workflow.RoundContext{Game: domain.Game{ID: world, Name: "Fixture valley", PlayerRole: domain.PlayerRoleObserver}, Round: domain.Round{Kind: domain.RoundKindDiscovery}, Observation: &workflow.ObservationContext{VisualDescription: "A dry, pale stone in direct sunlight. No visible moss. The surrounding ground is dry."}, HistorianAgentID: ids[3], Gameplay: &g}
	for i, id := range ids {
		round.Agents = append(round.Agents, workflow.AgentContext{Agent: domain.Agent{ID: id, GameID: world, Type: kinds[i], DisplayName: string(kinds[i]), PersonalityPrompt: "You are a persistent character. Use the supplied evidence and earlier records; do not invent events."}})
		g.Agents = append(g.Agents, gameplay.AgentIdentity{ID: id, Name: string(kinds[i]), Type: kinds[i]})
		for _, other := range ids {
			if id != other {
				g.Relationships = append(g.Relationships, gameplay.Relationship{FromAgentID: id, ToAgentID: other, Trust: "neutral", Rapport: "neutral", Version: 1})
			}
		}
	}
	tradition := workflow.TraditionContext{Tradition: domain.Tradition{ID: "00000000-0000-4000-8000-000000000030", Title: "Compare marked stones", Description: "Study moss on damp and dry stones before deciding what markings mean.", Type: domain.TraditionTypeRitual, State: domain.TraditionStateAdopted, Version: 1}, SupporterIDs: ids[:3]}
	round.Traditions = []workflow.TraditionContext{tradition}
	g.TraditionIDs = []domain.ID{tradition.Tradition.ID}
	reactions := []workflow.AgentReaction{}
	rebuttals := []workflow.AgentRebuttal{}
	relationships := []gameplay.RelationshipProposal{}
	citations := []gameplay.Influence{}
	type completion struct {
		index  int
		result workflow.AgentResult
		err    error
	}
	phase := func(name string) []workflow.AgentResult {
		completed := make(chan completion, len(round.Agents))
		for i, agent := range round.Agents {
			go func(index int, agent workflow.AgentContext) {
				result, err := a.GenerateAgentPhase(ctx, AgentPhaseInput{Request: workflow.AgentRequest{RequestID: agent.Agent.ID, GameID: world, Agent: agent, PlayerRole: domain.PlayerRoleObserver, RoundKind: domain.RoundKindDiscovery, Observation: round.Observation, Traditions: round.Traditions, Gameplay: &g, Phase: name, Reactions: reactions}})
				completed <- completion{index, result, err}
			}(i, agent)
		}
		outputs := make([]workflow.AgentResult, len(round.Agents))
		failed := false
		for range round.Agents {
			result := <-completed
			if result.err != nil {
				t.Errorf("%s %s: %v", round.Agents[result.index].Agent.Type, name, result.err)
				failed = true
			}
			outputs[result.index] = result.result
		}
		if failed {
			t.FailNow()
		}
		return outputs
	}
	for _, result := range phase("reaction") {
		reactions = append(reactions, *result.Reaction)
		citations = append(citations, result.Reaction.Influences...)
	}
	for _, result := range phase("rebuttal") {
		rebuttals = append(rebuttals, *result.Rebuttal)
		relationships = append(relationships, result.Rebuttal.RelationshipProposals...)
		citations = append(citations, result.Rebuttal.Influences...)
	}
	// Retain the final contribution for a given actor/tradition/category, just as the coordinator does.
	unique := map[string]gameplay.Influence{}
	for _, c := range citations {
		unique[string(c.AgentID)+"/"+string(c.TraditionID)+"/"+c.Category] = c
	}
	citations = nil
	for _, c := range unique {
		citations = append(citations, c)
	}

	record, e := a.WriteHistorianRecord(ctx, HistorianInput{Context: round, Reactions: reactions, Rebuttals: rebuttals, RelationshipChanges: relationships, CultureCitations: citations})
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("Validated four reactions, four rebuttals and chronicle; %d mystery updates, %d relationship proposals, %d retained influences", len(record.Mysteries), len(relationships), len(record.Influences))
}
