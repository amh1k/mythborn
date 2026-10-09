// Package retrieval builds game-scoped model memory context from committed lore.
package retrieval

import (
	"context"
	"fmt"
	"strings"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
)

type Embedder interface {
	Embed(context.Context, string, bool) ([]float32, error)
}

type SearchIndex interface {
	SearchRelevantMemories(context.Context, domain.ID, domain.ID, string, []float32, int) ([]workflow.Memory, error)
}

type Retriever struct {
	Embedder Embedder
	Index    SearchIndex
	Limit    int
}

func (r Retriever) Relevant(ctx context.Context, gameID, agentID domain.ID, query string) ([]workflow.Memory, error) {
	query = strings.TrimSpace(query)
	if query == "" || r.Index == nil {
		return nil, nil
	}
	limit := r.Limit
	if limit < 1 || limit > 8 {
		limit = 5
	}
	var vector []float32
	if r.Embedder != nil {
		var err error
		vector, err = r.Embedder.Embed(ctx, query, true)
		if err != nil {
			// Retrieval is auxiliary; keyword and recent-history retrieval can still
			// work when the embedding provider is unavailable.
			vector = nil
		}
	}
	items, err := r.Index.SearchRelevantMemories(ctx, gameID, agentID, query, vector, limit)
	if err != nil {
		return nil, fmt.Errorf("search game memories: %w", err)
	}
	for i := range items {
		if len(items[i].Content) > 12000 {
			items[i].Content = items[i].Content[:12000]
		}
	}
	return items, nil
}
