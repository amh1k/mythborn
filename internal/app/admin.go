package app

import (
	"context"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/repository"
)

type Admin struct{ Store *repository.Store }

func (a Admin) authorize(p auth.Principal) error {
	if !p.IsAdmin() {
		return ErrForbidden
	}
	return nil
}
func (a Admin) Templates(ctx context.Context, p auth.Principal) ([]repository.AgentTemplate, error) {
	if err := a.authorize(p); err != nil {
		return nil, err
	}
	return a.Store.ListAgentTemplates(ctx)
}
func (a Admin) CreateTemplate(ctx context.Context, p auth.Principal, t repository.AgentTemplate) (repository.AgentTemplate, error) {
	if err := a.authorize(p); err != nil {
		return t, err
	}
	return a.Store.CreateAgentTemplate(ctx, p.AccountID, t)
}
func (a Admin) ActivateTemplate(ctx context.Context, p auth.Principal, id domain.ID) error {
	if err := a.authorize(p); err != nil {
		return err
	}
	return a.Store.ActivateAgentTemplate(ctx, id)
}
func (a Admin) Accounts(ctx context.Context, p auth.Principal) ([]repository.AdminAccount, error) {
	if err := a.authorize(p); err != nil {
		return nil, err
	}
	return a.Store.ListAccounts(ctx)
}
func (a Admin) SetAccountRole(ctx context.Context, p auth.Principal, id domain.ID, role domain.SystemRole) error {
	if err := a.authorize(p); err != nil {
		return err
	}
	return a.Store.SetAccountRole(ctx, id, role)
}
func (a Admin) Games(ctx context.Context, p auth.Principal) ([]domain.Game, error) {
	if err := a.authorize(p); err != nil {
		return nil, err
	}
	return a.Store.ListAllGames(ctx)
}
