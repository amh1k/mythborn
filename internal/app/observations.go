package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
	"github.com/amh1k/mythborn/internal/storage"
)

type Observations struct {
	Store  *repository.Store
	Photos storage.PhotoStorage
}

func (o Observations) Create(ctx context.Context, p auth.Principal, gameID domain.ID, filename string, data []byte, playerStatement, idem string, allowReuse bool) (domain.ID, domain.ID, domain.ID, error) {
	if len(data) == 0 || len(data) > 10<<20 || len(idem) < 8 || len(idem) > 180 {
		return "", "", "", ErrInvalid
	}
	game, err := gameAccess(ctx, o.Store, p, gameID)
	if err != nil {
		return "", "", "", err
	}
	if game.Status != domain.GameStatusActive {
		return "", "", "", fmt.Errorf("%w: game is not active", ErrConflict)
	}
	if game.PlayerRole == domain.PlayerRoleObserver && strings.TrimSpace(playerStatement) != "" {
		return "", "", "", ErrInvalid
	}
	if len(playerStatement) > 2000 {
		return "", "", "", ErrInvalid
	}
	contentType := sniffImage(data, filename)
	if contentType == "" {
		return "", "", "", ErrInvalid
	}
	observationID, err := repository.NewID()
	if err != nil {
		return "", "", "", err
	}
	roundID, err := repository.NewID()
	if err != nil {
		return "", "", "", err
	}
	commandID, err := repository.NewID()
	if err != nil {
		return "", "", "", err
	}
	path := fmt.Sprintf("games/%s/observations/%s", gameID, observationID)
	if o.Photos == nil {
		return "", "", "", errors.New("photo storage unavailable")
	}
	if err = o.Photos.Put(ctx, path, contentType, bytes.NewReader(data)); err != nil {
		return "", "", "", err
	}
	sum := sha256.Sum256(data)
	accountID := p.AccountID
	if p.IsAdmin() {
		accountID = game.OwnerAccountID
	}
	obs, round, command, err := o.Store.CreateDiscovery(ctx, repository.DiscoveryInput{ObservationID: observationID, RoundID: roundID, CommandID: commandID, GameID: gameID, AccountID: accountID, PhotoPath: path, SourceSHA256: hex.EncodeToString(sum[:]), MIMEType: contentType, ByteSize: int64(len(data)), PlayerStatement: strings.TrimSpace(playerStatement), IdempotencyKey: idem, AllowReuse: allowReuse})
	if err != nil || obs != observationID {
		_ = o.Photos.Delete(ctx, path)
	}
	return obs, round, command, err
}

func gameAccess(ctx context.Context, store *repository.Store, p auth.Principal, id domain.ID) (domain.Game, error) {
	game, err := store.GetGame(ctx, id)
	if err != nil {
		return domain.Game{}, err
	}
	if game.OwnerAccountID != p.AccountID && !p.IsAdmin() {
		return domain.Game{}, ErrForbidden
	}
	return game, nil
}

func sniffImage(data []byte, filename string) string {
	if len(data) < 12 {
		return ""
	}
	contentType := ""
	switch {
	case bytes.HasPrefix(data, []byte("\xff\xd8\xff")):
		contentType = "image/jpeg"
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		contentType = "image/png"
	case string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		contentType = "image/webp"
	}
	_ = filename
	return contentType
}
