// Package app contains authorization and product use cases.
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
)

var (
	ErrForbidden = errors.New("forbidden")
	ErrInvalid   = errors.New("invalid request")
	ErrConflict  = errors.New("conflict")
)

type Games struct{ Store *repository.Store }

func (g Games) Create(ctx context.Context, p auth.Principal, name string, role domain.PlayerRole, idempotencyKey string) (domain.Game, domain.ID, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 || len(idempotencyKey) < 8 || len(idempotencyKey) > 180 {
		return domain.Game{}, "", ErrInvalid
	}
	if role != domain.PlayerRoleObserver && role != domain.PlayerRoleGod && role != domain.PlayerRoleMessenger {
		return domain.Game{}, "", ErrInvalid
	}
	game, command, err := g.Store.CreateGame(ctx, p.AccountID, name, role, idempotencyKey)
	if errors.Is(err, repository.ErrMissingTemplates) {
		return domain.Game{}, "", fmt.Errorf("%w: starting agent templates are not configured", ErrConflict)
	}
	return game, command, err
}

func (g Games) List(ctx context.Context, p auth.Principal) ([]domain.Game, error) {
	return g.Store.ListGames(ctx, p.AccountID)
}

func (g Games) Get(ctx context.Context, p auth.Principal, id domain.ID) (domain.Game, error) {
	game, err := g.Store.GetGame(ctx, id)
	if err != nil {
		return domain.Game{}, err
	}
	if game.OwnerAccountID != p.AccountID && !p.IsAdmin() {
		return domain.Game{}, ErrForbidden
	}
	return game, nil
}

func (g Games) SetReview(ctx context.Context, p auth.Principal, id domain.ID, enabled bool) error {
	game, err := g.Get(ctx, p, id)
	if err != nil {
		return err
	}
	if game.Status != domain.GameStatusActive && game.Status != domain.GameStatusStarting {
		return fmt.Errorf("%w: game is not editable", ErrConflict)
	}
	return g.Store.SetReviewPhotoDescription(ctx, id, enabled)
}

func (g Games) End(ctx context.Context, p auth.Principal, id domain.ID, idem string) (domain.ID, error) {
	game, err := g.Get(ctx, p, id)
	if err != nil {
		return "", err
	}
	if len(idem) < 8 || len(idem) > 180 {
		return "", ErrInvalid
	}
	if game.Status != domain.GameStatusActive {
		return "", fmt.Errorf("%w: game is not active", ErrConflict)
	}
	return g.Store.RequestGameEnd(ctx, game, p.AccountID, idem)
}

func (g Games) Delete(ctx context.Context, p auth.Principal, id domain.ID) (domain.ID, error) {
	if _, err := g.Get(ctx, p, id); err != nil {
		return "", err
	}
	return g.Store.MarkGameDeleting(ctx, id, p.AccountID)
}
