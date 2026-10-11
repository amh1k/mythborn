package app

import (
	"context"

	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/gameplay"
	"github.com/amh1k/mythborn/internal/repository"
)

type MysteryPage struct {
	Mysteries  []gameplay.Mystery `json:"mysteries"`
	NextCursor string             `json:"next_cursor,omitempty"`
}
type RevisionPage struct {
	Revisions   []gameplay.Revision `json:"revisions"`
	NextVersion int64               `json:"next_version,omitempty"`
}

func revisionPage(rows []gameplay.Revision) RevisionPage {
	p := RevisionPage{Revisions: rows}
	if len(rows) > 25 {
		p.Revisions = rows[:25]
		p.NextVersion = rows[24].Version
	}
	return p
}
func (l Lore) Mysteries(ctx context.Context, p auth.Principal, game domain.ID, state, cursor string) (MysteryPage, error) {
	if _, err := gameAccess(ctx, l.Store, p, game); err != nil {
		return MysteryPage{}, err
	}
	rows, err := l.Store.ListMysteries(ctx, game, state, cursor)
	out := MysteryPage{Mysteries: rows}
	if len(rows) > 25 {
		out.Mysteries = rows[:25]
		out.NextCursor = string(rows[24].ID)
	}
	return out, err
}
func (l Lore) Mystery(ctx context.Context, p auth.Principal, game, id domain.ID, after int64) (any, error) {
	if _, err := gameAccess(ctx, l.Store, p, game); err != nil {
		return nil, err
	}
	m, err := l.Store.Mystery(ctx, game, id)
	if err != nil {
		return nil, err
	}
	rows, err := l.Store.GameplayRevisions(ctx, game, id, "mystery", after)
	return struct {
		Mystery gameplay.Mystery `json:"mystery"`
		RevisionPage
	}{m, revisionPage(rows)}, err
}
func (l Lore) Relationships(ctx context.Context, p auth.Principal, game, agent domain.ID) ([]gameplay.Relationship, error) {
	if _, err := gameAccess(ctx, l.Store, p, game); err != nil {
		return nil, err
	}
	if _, err := l.Store.ListBeliefs(ctx, game, agent); err != nil {
		return nil, err
	}
	return l.Store.Relationships(ctx, game, agent)
}
func (l Lore) RelationshipHistory(ctx context.Context, p auth.Principal, game, agent, id domain.ID, after int64) (RevisionPage, error) {
	rows, err := l.Relationships(ctx, p, game, agent)
	if err != nil {
		return RevisionPage{}, err
	}
	found := false
	for _, r := range rows {
		if r.ID == id {
			found = true
		}
	}
	if !found {
		return RevisionPage{}, repository.ErrNotFound
	}
	revisions, err := l.Store.GameplayRevisions(ctx, game, id, "relationship", after)
	return revisionPage(revisions), err
}
func (l Lore) TraditionDetail(ctx context.Context, p auth.Principal, game, id domain.ID, cursor string, after int64) (any, error) {
	traditions, err := l.Traditions(ctx, p, game)
	if err != nil {
		return nil, err
	}
	for _, t := range traditions {
		if t.Tradition.ID == id {
			influences, e := l.Store.TraditionInfluences(ctx, game, id, cursor)
			if e != nil {
				return nil, e
			}
			next := ""
			if len(influences) > 25 {
				next = string(influences[24].ID)
				influences = influences[:25]
			}
			revisions, e := l.Store.TraditionRevisions(ctx, game, id, after)
			nextVersion := int64(0)
			if len(revisions) > 25 {
				nextVersion = revisions[24].Version
				revisions = revisions[:25]
			}
			return map[string]any{"tradition": t.Tradition, "supporter_ids": t.SupporterIDs, "influences": influences, "next_cursor": next, "revisions": revisions, "next_version": nextVersion}, e
		}
	}
	return nil, repository.ErrNotFound
}
