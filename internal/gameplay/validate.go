package gameplay

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/amh1k/mythborn/internal/domain"
)

func TextOK(s string, max int, optional bool) bool {
	return (optional || strings.TrimSpace(s) != "") && utf8.ValidString(s) && len(s) <= max
}
func Normalize(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }), " ")
}

// Evidence vocabulary connects ordinary inflections and a few direct synonyms.
// It does not infer a supernatural or causal connection from the image.
func evidenceWords(text string) []string {
	words := strings.Fields(Normalize(text))
	for i, w := range words {
		switch w {
		case "wet", "damp", "moist", "moisture":
			words[i] = "moisture"
		case "rock", "rocks", "stones":
			words[i] = "stone"
		case "mossy":
			words[i] = "moss"
		case "rainy", "rainfall", "raining":
			words[i] = "rain"
		case "insects", "bugs", "bug":
			words[i] = "insect"
		case "leaves", "foliage":
			words[i] = "leaf"
		default:
			if len(w) > 4 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
				words[i] = strings.TrimSuffix(w, "s")
			}
		}
	}
	filtered := make([]string, 0, len(words))
	stop := map[string]bool{"with": true, "that": true, "this": true, "from": true, "into": true, "near": true, "have": true, "their": true, "when": true, "does": true, "could": true, "would": true, "should": true, "about": true, "before": true, "after": true, "where": true, "there": true, "only": true, "some": true, "around": true, "under": true}
	for _, w := range words {
		if len(w) > 3 && !stop[w] {
			filtered = append(filtered, w)
		}
	}
	return filtered
}

// SearchQuery expands the same vocabulary used by in-memory ranking for PostgreSQL retrieval.
func SearchQuery(text string) string {
	terms := make([]string, 0, 30)
	seen := map[string]bool{}
	for _, w := range evidenceWords(text) {
		variants := []string{w}
		switch w {
		case "moisture":
			variants = []string{"moisture", "wet", "damp", "moist"}
		case "stone":
			variants = []string{"stone", "rock"}
		case "moss":
			variants = []string{"moss", "mossy"}
		case "rain":
			variants = []string{"rain", "rainfall", "rainy"}
		case "insect":
			variants = []string{"insect", "bug"}
		case "leaf":
			variants = []string{"leaf", "leaves", "foliage"}
		}
		for _, v := range variants {
			if !seen[v] {
				terms = append(terms, v)
				seen[v] = true
				if len(terms) == 30 {
					return strings.Join(terms, " OR ")
				}
			}
		}
	}
	return strings.Join(terms, " OR ")
}
func Relevance(query, text string) int {
	words := map[string]bool{}
	for _, w := range evidenceWords(query) {
		if len(w) > 3 {
			words[w] = true
		}
	}
	seen := map[string]bool{}
	score := 0
	for _, w := range evidenceWords(text) {
		if words[w] && !seen[w] {
			seen[w] = true
			score++
		}
	}
	return score
}

func ValidateMysteries(ps []MysteryProposal, current Context, agentIDs []domain.ID) error {
	if len(ps) > 4 {
		return fmt.Errorf("too many mystery updates")
	}
	agents := map[domain.ID]bool{}
	for _, id := range agentIDs {
		agents[id] = true
	}
	old := map[domain.ID]Mystery{}
	for _, m := range current.Mysteries {
		old[m.ID] = m
	}
	seen := map[string]bool{}
	added := 0
	openDelta := 0
	for _, p := range ps {
		if !TextOK(p.Title, 180, false) || !TextOK(p.Question, 600, false) || !TextOK(p.EvidenceSummary, 1500, false) || !TextOK(p.NextObservation, 500, true) || !TextOK(p.Reason, 1000, false) || len(p.Explanations) < 1 || len(p.Explanations) > 3 {
			return fmt.Errorf("invalid mystery content")
		}
		if p.State != "open" && p.State != "resolved" {
			return fmt.Errorf("invalid mystery state")
		}
		key := string(p.MysteryID)
		if key == "" {
			added++
			openDelta++
			key = Normalize(p.Question)
			if p.State != "open" || p.ExpectedVersion != 0 {
				return fmt.Errorf("new mystery must be open")
			}
			for _, m := range current.Mysteries {
				if Normalize(m.Question) == key {
					return fmt.Errorf("duplicate mystery")
				}
			}
		} else {
			m, ok := old[p.MysteryID]
			if ok && m.State != p.State {
				if p.State == "open" {
					openDelta++
				} else {
					openDelta--
				}
			}
			if !ok || m.Version != p.ExpectedVersion || p.Question != m.Question {
				return fmt.Errorf("mystery not in supplied context or question changed")
			}
		}
		if seen[key] {
			return fmt.Errorf("duplicate mystery update")
		}
		seen[key] = true
		keys := map[string]bool{}
		for _, e := range p.Explanations {
			if !TextOK(e.Key, 60, false) || keys[e.Key] || !TextOK(e.Claim, 600, false) || len(e.AgentIDs) > 4 {
				return fmt.Errorf("invalid mystery explanation")
			}
			keys[e.Key] = true
			members := map[domain.ID]bool{}
			for _, id := range e.AgentIDs {
				if !agents[id] || members[id] {
					return fmt.Errorf("invalid mystery position")
				}
				members[id] = true
			}
		}
	}
	if added > 1 || current.OpenMysteryCount+openDelta > 3 {
		return fmt.Errorf("open mystery capacity reached")
	}
	return nil
}
func ValidateRelationships(ps []RelationshipProposal, current Context, actor domain.ID) error {
	if len(ps) > 2 {
		return fmt.Errorf("too many relationship changes")
	}
	seen := map[domain.ID]bool{}
	for _, p := range ps {
		if p.FromAgentID != actor || p.ToAgentID == actor || seen[p.ToAgentID] || !TextOK(p.Summary, 800, false) || !TextOK(p.Reason, 1000, false) {
			return fmt.Errorf("invalid relationship actor or content")
		}
		seen[p.ToAgentID] = true
		if p.Trust != "cautious" && p.Trust != "neutral" && p.Trust != "trusting" {
			return fmt.Errorf("invalid trust")
		}
		if p.Rapport != "tense" && p.Rapport != "neutral" && p.Rapport != "warm" {
			return fmt.Errorf("invalid rapport")
		}
		found := false
		for _, r := range current.Relationships {
			if r.FromAgentID == actor && r.ToAgentID == p.ToAgentID && r.Version == p.ExpectedVersion {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("relationship not in supplied context")
		}
	}
	return nil
}
func ValidateInfluences(values []Influence, ids []domain.ID, actor domain.ID, max int) error {
	if len(values) > max {
		return fmt.Errorf("too many tradition influences")
	}
	allowed := map[domain.ID]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	seen := map[string]bool{}
	for _, v := range values {
		key := string(v.TraditionID) + "/" + string(v.AgentID) + "/" + v.Category
		if !allowed[v.TraditionID] || actor != "" && v.AgentID != actor || seen[key] || !TextOK(v.Explanation, 1000, false) {
			return fmt.Errorf("invalid tradition influence")
		}
		seen[key] = true
		switch v.Category {
		case "interpretation", "precaution", "challenge", "suggestion":
		default:
			return fmt.Errorf("invalid influence category")
		}
	}
	return nil
}
