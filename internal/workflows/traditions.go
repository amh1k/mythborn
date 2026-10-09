package workflows

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
)

// ComputeTraditionChanges applies the separate deterministic three-of-four
// rule. Every candidate must have one explicit declaration from each agent.
func ComputeTraditionChanges(gameID, roundID domain.ID, current []workflow.TraditionContext, candidates []workflow.TraditionIdea, votes []workflow.TraditionSupport, agentIDs []domain.ID) ([]workflow.TraditionChange, error) {
	if len(agentIDs) != 4 {
		return nil, fmt.Errorf("tradition vote requires exactly four agents")
	}
	knownAgents := make(map[domain.ID]bool, 4)
	for _, id := range agentIDs {
		if id == "" || knownAgents[id] {
			return nil, fmt.Errorf("invalid founding agent set")
		}
		knownAgents[id] = true
	}
	type entry struct {
		id    domain.ID
		idea  workflow.TraditionIdea
		prior *workflow.TraditionContext
		votes map[domain.ID]workflow.TraditionSupport
	}
	entries := map[string]*entry{}
	for i := range current {
		item := &current[i]
		id := item.Tradition.ID
		if id == "" || item.Tradition.GameID != gameID {
			return nil, fmt.Errorf("invalid current tradition")
		}
		copy := *item
		entries[string(id)] = &entry{id: id, idea: workflow.TraditionIdea{TraditionID: &id, Type: item.Tradition.Type, Title: item.Tradition.Title, Description: item.Tradition.Description}, prior: &copy, votes: map[domain.ID]workflow.TraditionSupport{}}
	}
	for _, idea := range candidates {
		if strings.TrimSpace(idea.Title) == "" || strings.TrimSpace(idea.Description) == "" {
			return nil, fmt.Errorf("tradition candidate requires a title and description")
		}
		key := CandidateKey(idea)
		if idea.TraditionID != nil {
			key = string(*idea.TraditionID)
		}
		if entries[key] == nil {
			entries[key] = &entry{id: StableTraditionID(gameID, roundID, key), idea: idea, votes: map[domain.ID]workflow.TraditionSupport{}}
		}
	}
	for _, vote := range votes {
		if !knownAgents[vote.AgentID] {
			return nil, fmt.Errorf("tradition vote from unknown agent")
		}
		key := vote.CandidateKey
		if vote.TraditionID != nil {
			key = string(*vote.TraditionID)
		}
		item := entries[key]
		if item == nil {
			return nil, fmt.Errorf("tradition vote references an unknown candidate")
		}
		if _, exists := item.votes[vote.AgentID]; exists {
			return nil, fmt.Errorf("duplicate tradition vote")
		}
		item.votes[vote.AgentID] = vote
	}
	keys := make([]string, 0, len(entries))
	for key, item := range entries {
		if item.prior == nil && len(item.votes) == 0 {
			continue // New ideas without support never enter the culture ledger.
		}
		if len(item.votes) != len(agentIDs) {
			return nil, fmt.Errorf("tradition candidate %q requires one vote from each agent", item.idea.Title)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	changes := make([]workflow.TraditionChange, 0, len(keys))
	for _, key := range keys {
		item := entries[key]
		supporters := make([]domain.ID, 0, 4)
		reasons := make([]string, 0, 4)
		for _, agentID := range agentIDs {
			vote := item.votes[agentID]
			if vote.Support {
				supporters = append(supporters, agentID)
			}
			if strings.TrimSpace(vote.Reason) != "" {
				reasons = append(reasons, string(agentID)+": "+strings.TrimSpace(vote.Reason))
			}
		}
		var state domain.TraditionState
		switch {
		case len(supporters) >= 3:
			state = domain.TraditionStateAdopted
		case item.prior == nil && len(supporters) >= 1:
			state = domain.TraditionStateContested
		case item.prior == nil:
			continue
		case len(supporters) == 2:
			state = domain.TraditionStateContested
		case len(supporters) <= 1:
			state = domain.TraditionStateRetired
		}
		reason := strings.Join(reasons, "\n")
		if reason == "" {
			reason = fmt.Sprintf("%d of 4 agents supported this tradition", len(supporters))
		}
		if item.prior != nil && item.prior.Tradition.Type == item.idea.Type && item.prior.Tradition.Title == item.idea.Title && item.prior.Tradition.Description == item.idea.Description && item.prior.Tradition.State == state && sameIDs(item.prior.SupporterIDs, supporters) {
			continue
		}
		changes = append(changes, workflow.TraditionChange{TraditionID: item.id, Type: item.idea.Type, Title: item.idea.Title, Description: item.idea.Description, State: state, SupporterIDs: supporters, Reason: reason})
	}
	return changes, nil
}

func sameIDs(a, b []domain.ID) bool {
	if len(a) != len(b) {
		return false
	}
	left, right := append([]domain.ID(nil), a...), append([]domain.ID(nil), b...)
	sort.Slice(left, func(i, j int) bool { return left[i] < left[j] })
	sort.Slice(right, func(i, j int) bool { return right[i] < right[j] })
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func CandidateKey(idea workflow.TraditionIdea) string {
	if idea.TraditionID != nil {
		return string(*idea.TraditionID)
	}
	if strings.TrimSpace(idea.CandidateKey) != "" {
		return idea.CandidateKey
	}
	return strings.ToLower(strings.TrimSpace(string(idea.Type) + ":" + idea.Title))
}

func StableTraditionID(gameID, roundID domain.ID, key string) domain.ID {
	sum := sha256.Sum256([]byte("mythborn/tradition/" + string(gameID) + "/" + string(roundID) + "/" + key))
	b := sum[:16]
	b[6] = b[6]&0x0f | 0x50
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return domain.ID(h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:])
}
