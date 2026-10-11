package workflows

import (
	"sort"

	"github.com/amh1k/mythborn/internal/gameplay"
	"github.com/amh1k/mythborn/internal/workflow"
)

// Sorting gives replay-stable consolidation despite parallel completion order.
func continuityContributions(reactions []workflow.AgentReaction, rebuttals []workflow.AgentRebuttal) ([]gameplay.RelationshipProposal, []gameplay.Influence) {
	relationships := make([]gameplay.RelationshipProposal, 0)
	byKey := map[string]gameplay.Influence{}
	add := func(values []gameplay.Influence) {
		for _, v := range values {
			key := string(v.AgentID) + "/" + string(v.TraditionID) + "/" + v.Category
			byKey[key] = v
		}
	}
	for _, r := range reactions {
		add(r.Influences)
	}
	for _, r := range rebuttals {
		relationships = append(relationships, r.RelationshipProposals...)
		add(r.Influences)
	}
	sort.Slice(relationships, func(i, j int) bool {
		a, b := relationships[i], relationships[j]
		if a.FromAgentID != b.FromAgentID {
			return a.FromAgentID < b.FromAgentID
		}
		return a.ToAgentID < b.ToAgentID
	})
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	citations := make([]gameplay.Influence, 0, len(keys))
	for _, key := range keys {
		citations = append(citations, byKey[key])
	}
	return relationships, citations
}
