package activities

import (
	"context"
	"strings"
	"testing"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/gameplay"
	"github.com/amh1k/mythborn/internal/workflow"
)

type continuityStore struct {
	Store
	context gameplay.Context
}

func (s continuityStore) LoadGameplayContext(context.Context, domain.ID, string) (gameplay.Context, error) {
	return s.context, nil
}
func TestCultureSelectionRelevantRetiredAndUnrelated(t *testing.T) {
	a := Activities{Store: continuityStore{context: gameplay.Context{ContractVersion: 1}}}
	current := workflow.RoundContext{Round: domain.Round{Kind: domain.RoundKindDiscovery}, Observation: &workflow.ObservationContext{VisualDescription: "A stone with moss in wet shade."}, Traditions: []workflow.TraditionContext{
		{Tradition: domain.Tradition{ID: "old", Title: "Marked moss stones", Description: "Observe wet shade", State: domain.TraditionStateRetired}},
		{Tradition: domain.Tradition{ID: "irrelevant", Title: "Star festival", Description: "Observe bright stars", State: domain.TraditionStateAdopted}},
	}}
	g, err := a.LoadGameplay(context.Background(), current)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.TraditionIDs) != 1 || g.TraditionIDs[0] != "old" {
		t.Fatalf("relevant retired culture missing or unrelated culture forced: %+v", g.TraditionIDs)
	}
}
func TestHistorianCannotInventCultureCitation(t *testing.T) {
	citation := gameplay.Influence{TraditionID: "ritual", AgentID: "soldier", Category: "challenge", Explanation: "This evidence challenges the taboo."}
	g := gameplay.Context{ContractVersion: 1, Features: gameplay.Enabled(), TraditionIDs: []domain.ID{"ritual"}}
	input := HistorianInput{Context: workflow.RoundContext{Gameplay: &g, Round: domain.Round{Kind: domain.RoundKindDiscovery}}, CultureCitations: []gameplay.Influence{citation}}
	output := workflow.HistorianRecord{Influences: []gameplay.Influence{citation}}
	if err := validateGameplayHistorian(input, output); err != nil {
		t.Fatal(err)
	}
	output.Influences[0].Explanation = "A festival actually happened."
	if validateGameplayHistorian(input, output) == nil {
		t.Fatal("invented enactment accepted")
	}
}
func TestDisabledGameplayRejectsChanges(t *testing.T) {
	g := gameplay.Context{ContractVersion: 1}
	req := workflow.AgentRequest{Gameplay: &g}
	if validateGameplayAgent(req, nil, []gameplay.Influence{{Explanation: "made up"}}) == nil {
		t.Fatal("disabled culture changed")
	}
}

func TestClosingContextRetainsOpenQuestionsAndCurrentCulture(t *testing.T) {
	a := Activities{Store: continuityStore{context: gameplay.Context{ContractVersion: 1, Mysteries: []gameplay.Mystery{{ID: "unresolved", State: "open"}}}}}
	current := workflow.RoundContext{Round: domain.Round{Kind: domain.RoundKindClosing}, Traditions: []workflow.TraditionContext{{Tradition: domain.Tradition{ID: "old-practice", Title: "Star festival", State: domain.TraditionStateAdopted}}}}
	g, err := a.LoadGameplay(context.Background(), current)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Mysteries) != 1 || len(g.TraditionIDs) != 1 {
		t.Fatal("closing chronicle lost continuity context")
	}
}

func TestCouncilGetsExistingCultureWithoutInventingPhotoEvidence(t *testing.T) {
	a := Activities{Store: continuityStore{context: gameplay.Context{ContractVersion: 1}}}
	current := workflow.RoundContext{Round: domain.Round{Kind: domain.RoundKindCouncil}, Traditions: []workflow.TraditionContext{{Tradition: domain.Tradition{ID: "contested", Title: "Star festival", State: domain.TraditionStateContested}}}}
	g, err := a.LoadGameplay(context.Background(), current)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.TraditionIDs) != 1 {
		t.Fatal("council lost the culture it is meant to reconsider")
	}
}

func TestPromptBudgetPreservesEvidenceCandidatesAndFrozenContext(t *testing.T) {
	original := gameplay.Context{Mysteries: []gameplay.Mystery{{ID: "question", Question: "Does moss follow moisture?", EvidenceSummary: strings.Repeat("evidence ", 300)}}}
	data := map[string]any{"evidence": "Visible stone", "candidates": []workflow.TraditionIdea{{CandidateKey: "candidate"}}, "gameplay_context": &original, "memories": []workflow.Memory{{Content: strings.Repeat("history ", 7000)}}}
	fitGameplayPrompt(data)
	if len(encodePrompt(data)) > 44*1024 {
		t.Fatal("optional history exceeded budget")
	}
	if data["evidence"] != "Visible stone" || data["candidates"].([]workflow.TraditionIdea)[0].CandidateKey != "candidate" {
		t.Fatal("mandatory evidence or vote IDs removed")
	}
	if len(original.Mysteries[0].EvidenceSummary) != len(strings.Repeat("evidence ", 300)) {
		t.Fatal("frozen context mutated")
	}
}
