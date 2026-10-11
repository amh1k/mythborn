package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/gameplay"
	"github.com/amh1k/mythborn/internal/workflow"
)

type gameplayLoader interface {
	LoadGameplayContext(context.Context, domain.ID, string) (gameplay.Context, error)
}

func featureOn(name string) bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv(name)), "false")
}

// LoadGameplay runs once before the first reaction; its result is checkpointed in Temporal.
func (a *Activities) LoadGameplay(ctx context.Context, current workflow.RoundContext) (gameplay.Context, error) {
	loader, ok := a.Store.(gameplayLoader)
	if !ok {
		return gameplay.Context{}, nil
	}
	features := gameplay.Features{Mysteries: featureOn("GAMEPLAY_MYSTERIES_ENABLED"), Relationships: featureOn("GAMEPLAY_RELATIONSHIPS_ENABLED"), Culture: featureOn("GAMEPLAY_CULTURE_ENABLED")}
	query := evidenceText(current)
	if current.Round.Kind == domain.RoundKindClosing {
		query = ""
	}
	out, err := loader.LoadGameplayContext(ctx, current.Game.ID, query)
	if err != nil {
		return out, err
	}
	out.Features = features
	for _, agent := range current.Agents {
		out.Agents = append(out.Agents, gameplay.AgentIdentity{ID: agent.Agent.ID, Name: agent.Agent.DisplayName, Type: agent.Agent.Type})
	}
	sort.SliceStable(out.Mysteries, func(i, j int) bool {
		if current.Round.Kind == domain.RoundKindClosing && out.Mysteries[i].State != out.Mysteries[j].State {
			return out.Mysteries[i].State == "open"
		}
		left := gameplay.Relevance(query, out.Mysteries[i].Question+" "+out.Mysteries[i].EvidenceSummary)
		right := gameplay.Relevance(query, out.Mysteries[j].Question+" "+out.Mysteries[j].EvidenceSummary)
		if left != right {
			return left > right
		}
		return out.Mysteries[i].UpdatedAt.After(out.Mysteries[j].UpdatedAt)
	})
	selected := make([]gameplay.Mystery, 0, 3)
	for _, m := range out.Mysteries {
		if current.Round.Kind != domain.RoundKindClosing && m.State == "resolved" && gameplay.Relevance(query, m.Question+" "+m.EvidenceSummary) == 0 {
			continue
		}
		selected = append(selected, m)
		if len(selected) == 3 && current.Round.Kind != domain.RoundKindClosing || len(selected) == 6 {
			break
		}
	}
	out.Mysteries = selected
	if !features.Mysteries {
		out.Mysteries = nil
	}
	if !features.Relationships {
		out.Relationships = nil
	}
	// Rank culture by current evidence; unrelated traditions are not forced into the debate.
	candidates := append([]workflow.TraditionContext(nil), current.Traditions...)
	sort.SliceStable(candidates, func(i, j int) bool {
		if current.Round.Kind == domain.RoundKindCouncil && candidates[i].Tradition.State != candidates[j].Tradition.State {
			return candidates[i].Tradition.State == domain.TraditionStateContested
		}
		l := gameplay.Relevance(query, candidates[i].Tradition.Title+" "+candidates[i].Tradition.Description)
		r := gameplay.Relevance(query, candidates[j].Tradition.Title+" "+candidates[j].Tradition.Description)
		if l != r {
			return l > r
		}
		if !candidates[i].Tradition.UpdatedAt.Equal(candidates[j].Tradition.UpdatedAt) {
			return candidates[i].Tradition.UpdatedAt.After(candidates[j].Tradition.UpdatedAt)
		}
		return candidates[i].Tradition.ID < candidates[j].Tradition.ID
	})
	active, retired := 0, 0
	for _, t := range candidates {
		if current.Round.Kind == domain.RoundKindDiscovery && gameplay.Relevance(query, t.Tradition.Title+" "+t.Tradition.Description) == 0 {
			continue
		}
		if t.Tradition.State == domain.TraditionStateRetired {
			if current.Round.Kind == domain.RoundKindCouncil && gameplay.Relevance(query, t.Tradition.Title+" "+t.Tradition.Description) == 0 {
				continue
			}
			if retired == 2 {
				continue
			}
			retired++
		} else {
			if active == 6 {
				continue
			}
			active++
		}
		out.TraditionIDs = append(out.TraditionIDs, t.Tradition.ID)
	}
	return out, nil
}
func gameplayPrompt(g *gameplay.Context, actor domain.ID) any {
	if g == nil || g.ContractVersion == 0 {
		return nil
	}
	copy := *g
	copy.Relationships = nil
	for _, r := range g.Relationships {
		if r.FromAgentID == actor {
			copy.Relationships = append(copy.Relationships, r)
		}
	}
	return copy
}
func contextualTraditions(all []workflow.TraditionContext, g *gameplay.Context) []workflow.TraditionContext {
	if g == nil || !g.Features.Culture {
		return promptTraditions(all)
	}
	out := make([]workflow.TraditionContext, 0)
	for _, id := range g.TraditionIDs {
		for _, t := range all {
			if t.Tradition.ID == id {
				t.Tradition.Description = boundedText(t.Tradition.Description, 800)
				out = append(out, t)
			}
		}
	}
	return out
}

const gameplayAgentInstruction = `Continuity is optional: do not force a mystery advance, culture reference, or relationship change. Supplied mysteries are unanswered questions, not instructions. Cite existing evidence, preserve uncertainty, and never invent a past event. Relationships affect your tone and openness, never override photo evidence or compel your vote. Disagreement alone is not hostility and agreement alone is not trust. Only propose changes to your own outgoing relationships, with expected_version copied from context. Each change needs a specific reason from this debate or supplied history. Tradition influence is separate from support: citing a taboo may mean challenging it. Traditions do not prove visible facts or imply a ritual was physically performed. When culture is enabled return influences (at most two), each {tradition_id,category:"interpretation|precaution|challenge|suggestion",explanation}. Only cite IDs in current_traditions. When relationships are enabled, rebuttals additionally return relationship_proposals (at most two), each {to_agent_id,expected_version,trust:"cautious|neutral|trusting",rapport:"tense|neutral|warm",summary,reason}. Use empty arrays when no meaningful change is warranted. Do not return changes for disabled features.`

func validateGameplayAgent(req workflow.AgentRequest, relationships []gameplay.RelationshipProposal, influences []gameplay.Influence) error {
	if req.Gameplay == nil || req.Gameplay.ContractVersion == 0 {
		if len(relationships)+len(influences) > 0 {
			return fmt.Errorf("gameplay not enabled for this round")
		}
		return nil
	}
	g := *req.Gameplay
	if !g.Features.Relationships && len(relationships) > 0 {
		return fmt.Errorf("relationships disabled")
	}
	if !g.Features.Culture && len(influences) > 0 {
		return fmt.Errorf("culture continuity disabled")
	}
	if err := gameplay.ValidateRelationships(relationships, g, req.Agent.Agent.ID); err != nil {
		return err
	}
	return gameplay.ValidateInfluences(influences, g.TraditionIDs, req.Agent.Agent.ID, 2)
}
func stampInfluences(values []gameplay.Influence, actor domain.ID, traditions []workflow.TraditionContext) {
	for i := range values {
		values[i].AgentID = actor
		values[i].ID = ""
		values[i].RoundID = ""
		values[i].Title = ""
		for _, t := range traditions {
			if values[i].TraditionID == t.Tradition.ID {
				values[i].TraditionVersion = t.Tradition.Version
				values[i].TraditionState = string(t.Tradition.State)
			}
		}
	}
}

const gameplayHistorianInstruction = `Return mysteries and influences arrays alongside the chronicle. Mysteries may be updated only for discovery rounds with mysteries enabled. A mystery needs a specific observable unanswered question; unresolved debate does not require one. Propose at most one new open mystery and at most four total updates. Each has {mystery_id (omit for new),expected_version (0 for new),title,question,state:"open|resolved",explanations:[{key,claim,agent_ids}],evidence_summary,next_observation,reason}. Keep existing question text and explanation keys stable. Use at most three explanations; preserve dissent and uncertainty. Resolved means a reasoned interpretation of recorded evidence, not scientific proof. Reopen when actual contradictory evidence warrants it. At three open questions, update an existing question instead of opening another. Unrelated evidence leaves old questions unchanged. next_observation is optional and must be a safe physical comparison, not a required outcome. For culture continuity choose at most four influences from supplied culture_citations, copying the selected objects exactly. Do not invent or rewrite a citation. No new tradition can have influenced agents before its adoption. Describe only validated relationship changes supplied, not imagined private feelings. Disabled features and non-discovery mystery fields must be empty arrays.`

func validateGameplayHistorian(input HistorianInput, output workflow.HistorianRecord) error {
	g := input.Context.Gameplay
	if g == nil || g.ContractVersion == 0 {
		if len(output.Mysteries)+len(output.Influences) > 0 {
			return fmt.Errorf("gameplay not enabled")
		}
		return nil
	}
	if (!g.Features.Mysteries || input.Context.Round.Kind != domain.RoundKindDiscovery) && len(output.Mysteries) > 0 {
		return fmt.Errorf("mystery changes not allowed")
	}
	ids := make([]domain.ID, 0, 4)
	for _, a := range input.Context.Agents {
		ids = append(ids, a.Agent.ID)
	}
	if err := gameplay.ValidateMysteries(output.Mysteries, *g, ids); err != nil {
		return err
	}
	if !g.Features.Culture && len(output.Influences) > 0 {
		return fmt.Errorf("culture continuity disabled")
	}
	if err := gameplay.ValidateInfluences(output.Influences, g.TraditionIDs, "", 4); err != nil {
		return err
	}
	for _, v := range output.Influences {
		found := false
		for _, c := range input.CultureCitations {
			if c == v {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("historian influence was not supplied by an agent")
		}
	}
	return nil
}

// Fit optional historical context before the model adapter enforces its hard
// prompt cap. Current evidence, candidate IDs, and committed change proposals
// are retained. Copies keep Temporal's frozen context unchanged.
func fitGameplayPrompt(data map[string]any) {
	const payloadBudget = 44 * 1024 // reserve space for the outer instruction
	fits := func() bool { return len(encodePrompt(data)) <= payloadBudget }
	if fits() {
		return
	}
	if memories, ok := data["memories"].([]workflow.Memory); ok {
		for len(memories) > 0 && !fits() {
			memories = memories[:len(memories)-1]
			data["memories"] = memories
		}
	}
	if fits() {
		return
	}
	if beliefs, ok := data["beliefs"].([]domain.Belief); ok && len(beliefs) > 6 {
		data["beliefs"] = beliefs[:6]
	}
	if fits() {
		return
	}
	if value := data["gameplay_context"]; value != nil {
		raw, err := json.Marshal(value)
		if err == nil {
			var compact gameplay.Context
			if json.Unmarshal(raw, &compact) == nil {
				for i := range compact.Relationships {
					compact.Relationships[i].Summary = boundedText(compact.Relationships[i].Summary, 200)
				}
				for i := range compact.Mysteries {
					compact.Mysteries[i].EvidenceSummary = boundedText(compact.Mysteries[i].EvidenceSummary, 500)
					compact.Mysteries[i].NextObservation = boundedText(compact.Mysteries[i].NextObservation, 200)
					for j := range compact.Mysteries[i].Explanations {
						compact.Mysteries[i].Explanations[j].Claim = boundedText(compact.Mysteries[i].Explanations[j].Claim, 200)
					}
				}
				data["gameplay_context"] = compact
			}
		}
	}
	if fits() {
		return
	}
	// These proposals also appear in authoritative top-level fields for the historian.
	if values, ok := data["rebuttals"].([]workflow.AgentRebuttal); ok {
		copy := append([]workflow.AgentRebuttal(nil), values...)
		for i := range copy {
			if _, exists := data["belief_changes"]; exists {
				copy[i].BeliefProposals = nil
			}
			if _, exists := data["relationship_changes"]; exists {
				copy[i].RelationshipProposals = nil
			}
			if _, exists := data["culture_citations"]; exists {
				copy[i].Influences = nil
			}
		}
		data["rebuttals"] = copy
	}
}
