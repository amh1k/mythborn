package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
)

type debateStoreStub struct {
	round domain.Round
	game  domain.Game
}

func (s debateStoreStub) GetRound(context.Context, domain.ID) (domain.Round, error) {
	return s.round, nil
}
func (s debateStoreStub) GetGame(context.Context, domain.ID) (domain.Game, error) { return s.game, nil }

type debateReaderStub struct {
	calls   int
	preview *workflow.DebatePreview
	err     error
}

func (s *debateReaderStub) Read(ctx context.Context, _, _ domain.ID) (*workflow.DebatePreview, error) {
	s.calls++
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 1600*time.Millisecond {
		panic("preview query must have a short timeout")
	}
	return s.preview, s.err
}

func TestDebateAccessAndFallback(t *testing.T) {
	for _, role := range []domain.PlayerRole{domain.PlayerRoleObserver, domain.PlayerRoleGod, domain.PlayerRoleMessenger} {
		t.Run(string(role), func(t *testing.T) {
			store := debateStoreStub{round: domain.Round{ID: "round", GameID: "game", Status: domain.RoundStatusRunning, Stage: domain.RoundStageReacting}, game: domain.Game{ID: "game", OwnerAccountID: "owner", PlayerRole: role, Status: domain.GameStatusActive}}
			reader := &debateReaderStub{preview: &workflow.DebatePreview{GameID: "game", RoundID: "round"}}
			service := Debates{Store: store, Reader: reader}
			if _, err := service.Get(context.Background(), auth.Principal{AccountID: "stranger"}, "round"); !errors.Is(err, ErrForbidden) || reader.calls != 0 {
				t.Fatal("another owner reached the live workflow")
			}
			for _, principal := range []auth.Principal{{AccountID: "owner"}, {AccountID: "admin", SystemRole: domain.SystemRoleAdmin}} {
				detail, err := service.Get(context.Background(), principal, "round")
				if err != nil || detail.State != "live" || detail.PlayerRole != role || detail.Preview == nil {
					t.Fatalf("owner/admin cannot watch: %+v %v", detail, err)
				}
			}
			reader.err = errors.New("Temporal unavailable")
			detail, err := service.Get(context.Background(), auth.Principal{AccountID: "owner"}, "round")
			if err != nil || detail.State != "unavailable" || detail.Preview != nil {
				t.Fatal("Temporal outage must not break durable reads")
			}
			reader.err = nil
			reader.preview.RoundID = "another-round"
			detail, _ = service.Get(context.Background(), auth.Principal{AccountID: "owner"}, "round")
			if detail.Preview != nil {
				t.Fatal("preview from another round leaked")
			}
		})
	}
}

func TestDebateDoesNotRestoreCompletedTranscripts(t *testing.T) {
	for _, status := range []domain.RoundStatus{domain.RoundStatusComplete, domain.RoundStatusAbandoned} {
		reader := &debateReaderStub{preview: &workflow.DebatePreview{GameID: "game", RoundID: "round"}}
		service := Debates{Store: debateStoreStub{round: domain.Round{ID: "round", GameID: "game", Status: status, Stage: domain.RoundStageWriting}, game: domain.Game{ID: "game", OwnerAccountID: "owner"}}, Reader: reader}
		detail, err := service.Get(context.Background(), auth.Principal{AccountID: "owner"}, "round")
		if err != nil || detail.State != "finished" || detail.Preview != nil || reader.calls != 0 {
			t.Fatal("completed or abandoned round queried a raw transcript")
		}
	}
}
