package app

import (
	"context"
	"fmt"

	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
)

type EpisodeDetail struct {
	Episode     domain.Round      `json:"episode"`
	Observation any               `json:"observation,omitempty"`
	Chronicle   *domain.Chronicle `json:"chronicle,omitempty"`
}

type Episodes struct{ Store *repository.Store }

func (e Episodes) Get(ctx context.Context, p auth.Principal, id domain.ID) (EpisodeDetail, error) {
	round, err := e.Store.GetRound(ctx, id)
	if err != nil {
		return EpisodeDetail{}, err
	}
	if _, err = gameAccess(ctx, e.Store, p, round.GameID); err != nil {
		return EpisodeDetail{}, err
	}
	detail := EpisodeDetail{Episode: round}
	if round.ObservationID != nil {
		observation, eerr := e.Store.GetObservation(ctx, round.GameID, *round.ObservationID)
		if eerr != nil {
			return EpisodeDetail{}, eerr
		}
		detail.Observation = map[string]any{"id": observation.ID, "description": observation.VisualDescription, "player_correction": observation.PlayerCorrection, "description_status": observation.DescriptionStatus}
	}
	detail.Chronicle, err = e.Store.GetChronicle(ctx, id)
	if err != nil {
		return EpisodeDetail{}, err
	}
	return detail, nil
}

func (e Episodes) DecideDescription(ctx context.Context, p auth.Principal, roundID domain.ID, decision, correction, clarityChoice, key string) (domain.ID, error) {
	round, err := e.Store.GetRound(ctx, roundID)
	if err != nil {
		return "", err
	}
	game, err := gameAccess(ctx, e.Store, p, round.GameID)
	if err != nil {
		return "", err
	}
	accountID := p.AccountID
	if p.IsAdmin() {
		accountID = game.OwnerAccountID
	}
	id, err := e.Store.SubmitDescriptionDecision(ctx, accountID, round.GameID, roundID, decision, correction, clarityChoice, key)
	if err != nil {
		return "", translateConflict(err)
	}
	return id, nil
}

func (e Episodes) Retry(ctx context.Context, p auth.Principal, roundID domain.ID, key string) (domain.ID, error) {
	round, err := e.Store.GetRound(ctx, roundID)
	if err != nil {
		return "", err
	}
	game, err := gameAccess(ctx, e.Store, p, round.GameID)
	if err != nil {
		return "", err
	}
	accountID := p.AccountID
	if p.IsAdmin() {
		accountID = game.OwnerAccountID
	}
	id, err := e.Store.RetryRound(ctx, accountID, round.GameID, roundID, key)
	if err != nil {
		return "", translateConflict(err)
	}
	return id, nil
}

func translateConflict(err error) error {
	if err == repository.ErrStateConflict {
		return fmt.Errorf("%w: %v", ErrConflict, err)
	}
	return err
}
