package app

import (
	"context"
	"fmt"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
	"github.com/amh1k/mythborn/internal/workflow"
)

type Messenger struct{ Store *repository.Store }

func (m Messenger) Thread(ctx context.Context, p auth.Principal, roundID, agentID domain.ID) (workflow.MessengerContext, error) {
	round, err := m.Store.GetRound(ctx, roundID)
	if err != nil {
		return workflow.MessengerContext{}, err
	}
	game, err := gameAccess(ctx, m.Store, p, round.GameID)
	if err != nil {
		return workflow.MessengerContext{}, err
	}
	if game.PlayerRole != domain.PlayerRoleMessenger {
		return workflow.MessengerContext{}, ErrForbidden
	}
	return m.Store.LoadMessengerContext(ctx, game.ID, round.ID, agentID)
}

func (m Messenger) Send(ctx context.Context, p auth.Principal, roundID, agentID domain.ID, message, key string, expected int64) (domain.ID, error) {
	thread, err := m.Thread(ctx, p, roundID, agentID)
	if err != nil {
		return "", err
	}
	if len(key) < 8 || len(key) > 180 {
		return "", ErrInvalid
	}
	if thread.Game.Status != domain.GameStatusActive {
		return "", fmt.Errorf("%w: game is not active", ErrConflict)
	}
	accountID := p.AccountID
	if p.IsAdmin() {
		accountID = thread.Game.OwnerAccountID
	}
	id, err := m.Store.CreateMessengerCommand(ctx, accountID, thread.Game.ID, roundID, agentID, message, key, expected)
	if err == repository.ErrStateConflict {
		return "", fmt.Errorf("%w: %v", ErrConflict, err)
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

type Commands struct{ Store *repository.Store }

func (c Commands) Get(ctx context.Context, p auth.Principal, id domain.ID) (domain.WorkflowCommand, error) {
	return c.Store.GetCommand(ctx, id, p.AccountID, p.IsAdmin())
}
