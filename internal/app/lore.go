package app

import (
	"context"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
	"github.com/amh1k/mythborn/internal/workflow"
)

type Lore struct{ Store *repository.Store }

func (l Lore) Agents(ctx context.Context, p auth.Principal, gameID domain.ID) ([]domain.Agent, error) {
	if _, err := gameAccess(ctx, l.Store, p, gameID); err != nil {
		return nil, err
	}
	return l.Store.ListAgents(ctx, gameID)
}
func (l Lore) Beliefs(ctx context.Context, p auth.Principal, gameID, agentID domain.ID) ([]domain.Belief, []domain.BeliefRevision, error) {
	if _, err := gameAccess(ctx, l.Store, p, gameID); err != nil {
		return nil, nil, err
	}
	beliefs, err := l.Store.ListBeliefs(ctx, gameID, agentID)
	if err != nil {
		return nil, nil, err
	}
	revisions, err := l.Store.ListBeliefRevisions(ctx, gameID, agentID)
	return beliefs, revisions, err
}
func (l Lore) Traditions(ctx context.Context, p auth.Principal, gameID domain.ID) ([]workflow.TraditionContext, error) {
	if _, err := gameAccess(ctx, l.Store, p, gameID); err != nil {
		return nil, err
	}
	return l.Store.ListTraditions(ctx, gameID)
}
func (l Lore) History(ctx context.Context, p auth.Principal, gameID domain.ID) ([]domain.Chronicle, error) {
	if _, err := gameAccess(ctx, l.Store, p, gameID); err != nil {
		return nil, err
	}
	return l.Store.ListChronicles(ctx, gameID)
}
