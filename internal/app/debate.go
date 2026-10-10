package app

import (
	"context"
	"time"

	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
)

type DebateStore interface {
	GetRound(context.Context, domain.ID) (domain.Round, error)
	GetGame(context.Context, domain.ID) (domain.Game, error)
}

type Debates struct {
	Store  DebateStore
	Reader workflow.DebatePreviewReader
}

type DebateDetail struct {
	State      string                  `json:"state"`
	PlayerRole domain.PlayerRole       `json:"player_role"`
	Preview    *workflow.DebatePreview `json:"preview"`
}

// Get authorizes the owner/admin before consulting the live workflow. Watching
// is allowed for all game roles; it does not grant any messaging capability.
func (d Debates) Get(ctx context.Context, p auth.Principal, roundID domain.ID) (DebateDetail, error) {
	round, err := d.Store.GetRound(ctx, roundID)
	if err != nil {
		return DebateDetail{}, err
	}
	game, err := d.Store.GetGame(ctx, round.GameID)
	if err != nil {
		return DebateDetail{}, err
	}
	if game.OwnerAccountID != p.AccountID && !p.IsAdmin() {
		return DebateDetail{}, ErrForbidden
	}
	detail := DebateDetail{State: "waiting", PlayerRole: game.PlayerRole}
	if round.Status == domain.RoundStatusComplete || round.Status == domain.RoundStatusAbandoned || game.Status == domain.GameStatusDeleting || game.Status == domain.GameStatusArchived {
		detail.State = "finished"
		return detail, nil
	}
	if round.Stage != domain.RoundStageDescribing && round.Stage != domain.RoundStageReacting && round.Stage != domain.RoundStageDebating && round.Stage != domain.RoundStageWriting {
		return detail, nil
	}
	if d.Reader == nil {
		detail.State = "unavailable"
		return detail, nil
	}
	queryCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	preview, err := d.Reader.Read(queryCtx, game.ID, round.ID)
	if err != nil {
		// Durable episode polling and retries remain usable if Temporal is down.
		detail.State = "unavailable"
		return detail, nil
	}
	if preview != nil {
		if preview.GameID != game.ID || preview.RoundID != round.ID {
			detail.State = "unavailable"
			return detail, nil
		}
		detail.State, detail.Preview = "live", preview
	}
	return detail, nil
}
