package app

import (
	"context"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
)

type Accounts struct{ Store *repository.Store }

func (a Accounts) Delete(ctx context.Context, p auth.Principal) (domain.ID, domain.ID, error) {
	return a.Store.RequestAccountDeletion(ctx, p.AccountID)
}
