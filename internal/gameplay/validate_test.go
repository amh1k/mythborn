package gameplay

import (
	"testing"

	"github.com/amh1k/mythborn/internal/domain"
)

func mysteryProposal() MysteryProposal {
	return MysteryProposal{Title: "Marked stones", Question: "Does moss follow moisture?", State: "open", EvidenceSummary: "A damp stone bears moss.", Explanations: []Explanation{{Key: "moisture", Claim: "Moss follows moisture.", AgentIDs: []domain.ID{"scientist"}}}, Reason: "The photograph leaves an observable question."}
}
func TestMysteryCapacityTransitionsAndAttribution(t *testing.T) {
	p := mysteryProposal()
	g := Context{OpenMysteryCount: 3}
	if ValidateMysteries([]MysteryProposal{p}, g, []domain.ID{"scientist"}) == nil {
		t.Fatal("fourth open mystery accepted")
	}
	old := Mystery{ID: "old", Question: p.Question, State: "open", Version: 2}
	g.Mysteries = []Mystery{old}
	resolve := p
	resolve.MysteryID = "old"
	resolve.ExpectedVersion = 2
	resolve.State = "resolved"
	newOne := p
	newOne.Question = "Do insects gather near the water?"
	if err := ValidateMysteries([]MysteryProposal{resolve, newOne}, g, []domain.ID{"scientist"}); err != nil {
		t.Fatal("resolving makes room", err)
	}
	p.Explanations[0].AgentIDs = []domain.ID{"foreign"}
	if ValidateMysteries([]MysteryProposal{p}, Context{}, []domain.ID{"scientist"}) == nil {
		t.Fatal("foreign agent accepted")
	}
	resolve.ExpectedVersion = 1
	if ValidateMysteries([]MysteryProposal{resolve}, g, []domain.ID{"scientist"}) == nil {
		t.Fatal("stale revision accepted")
	}
	old.State = "resolved"
	g.Mysteries = []Mystery{old}
	resolve.State = "open"
	resolve.ExpectedVersion = 2
	resolve.Explanations[0].AgentIDs = []domain.ID{"scientist"}
	if ValidateMysteries([]MysteryProposal{resolve}, g, []domain.ID{"scientist"}) == nil {
		t.Fatal("reopening exceeded capacity")
	}
}
func TestDirectedRelationshipValidation(t *testing.T) {
	g := Context{Relationships: []Relationship{{FromAgentID: "soldier", ToAgentID: "scientist", Version: 1}}}
	p := RelationshipProposal{FromAgentID: "soldier", ToAgentID: "scientist", ExpectedVersion: 1, Trust: "trusting", Rapport: "neutral", Summary: "Respects a careful comparison.", Reason: "The scientist separated evidence from fear."}
	if err := ValidateRelationships([]RelationshipProposal{p}, g, "soldier"); err != nil {
		t.Fatal(err)
	}
	if ValidateRelationships([]RelationshipProposal{p}, g, "scientist") == nil {
		t.Fatal("agent changed someone else's feelings")
	}
	p.ToAgentID = "soldier"
	if ValidateRelationships([]RelationshipProposal{p}, g, "soldier") == nil {
		t.Fatal("self relationship accepted")
	}
}
func TestInfluenceMustUseSuppliedCultureAndActor(t *testing.T) {
	v := Influence{TraditionID: "ritual", AgentID: "soldier", Category: "challenge", Explanation: "The new evidence challenges this practice."}
	if err := ValidateInfluences([]Influence{v}, []domain.ID{"ritual"}, "soldier", 2); err != nil {
		t.Fatal(err)
	}
	if ValidateInfluences([]Influence{v}, nil, "soldier", 2) == nil {
		t.Fatal("invented tradition accepted")
	}
	if ValidateInfluences([]Influence{v, v}, []domain.ID{"ritual"}, "soldier", 2) == nil {
		t.Fatal("duplicate mention persisted")
	}
}

func TestEvidenceVocabularyDoesNotLoseRelevantOldCulture(t *testing.T) {
	if Relevance("Mossy rocks in wet shade", "Compare moss on damp stones") < 3 {
		t.Fatal("direct synonyms lost continuity")
	}
	if Relevance("Mossy rocks in wet shade", "The star festival") != 0 {
		t.Fatal("unrelated culture became relevant")
	}
}
