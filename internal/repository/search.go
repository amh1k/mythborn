package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
)

func (s *Store) SearchRelevantMemories(ctx context.Context, gameID, agentID domain.ID, query string, embedding []float32, limit int) ([]workflow.Memory, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit < 1 || limit > 8 {
		limit = 5
	}
	var vector any
	if len(embedding) == 768 {
		vector = pgVectorLiteral(embedding)
	}
	sql := `SELECT CASE WHEN observation_id IS NOT NULL THEN 'observation' WHEN chronicle_id IS NOT NULL THEN 'chronicle' WHEN belief_id IS NOT NULL THEN 'belief' ELSE 'conversation_summary' END,
CASE WHEN observation_id IS NOT NULL THEN observation_id::text WHEN chronicle_id IS NOT NULL THEN chronicle_id::text WHEN belief_id IS NOT NULL THEN belief_id::text ELSE conversation_summary_id::text END,
content,created_at,
ts_rank_cd(to_tsvector('english',content),websearch_to_tsquery('english',$3)) AS text_score,
CASE WHEN $4::vector IS NULL OR embedding IS NULL THEN 0::real ELSE greatest(0,1-(embedding <=> $4::vector))::real END AS vector_score
FROM search_documents WHERE game_id=$1 AND (agent_id IS NULL OR agent_id=$2)
AND (to_tsvector('english',content) @@ websearch_to_tsquery('english',$3) OR ($4::vector IS NOT NULL AND embedding IS NOT NULL))
ORDER BY (text_score + vector_score) DESC,created_at DESC LIMIT $5`
	rows, err := s.DB.Query(ctx, sql, string(gameID), string(agentID), query, vector, limit)
	if err != nil {
		// Memory lookup is supplementary. Provider/index hiccups must not stop a round.
		return []workflow.Memory{}, nil
	}
	defer rows.Close()
	items := make([]workflow.Memory, 0, limit)
	for rows.Next() {
		var m workflow.Memory
		var sourceType, sourceID string
		var scoreText, scoreVector float32
		if err = rows.Scan(&sourceType, &sourceID, &m.Content, &m.CreatedAt, &scoreText, &scoreVector); err != nil {
			return []workflow.Memory{}, nil
		}
		m.SourceType, m.SourceID = sourceType, domain.ID(sourceID)
		if len(m.Content) > 12000 {
			m.Content = m.Content[:12000]
		}
		items = append(items, m)
	}
	if err = rows.Err(); err != nil {
		return []workflow.Memory{}, nil
	}
	return items, nil
}

func pgVectorLiteral(values []float32) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.FormatFloat(float64(value), 'f', 6, 32)
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ","))
}
