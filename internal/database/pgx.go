// Package database adapts pgxpool to the repository SQL contracts.
package database

import (
	"context"
	"errors"

	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Pool{pool: pool}, nil
}

func (p *Pool) BeginTx(ctx context.Context) (contracts.Tx, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return transaction{tx: tx}, nil
}

func (p *Pool) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	tag, err := p.pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (p *Pool) Query(ctx context.Context, query string, args ...any) (contracts.Rows, error) {
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (p *Pool) QueryRow(ctx context.Context, query string, args ...any) contracts.Row {
	return row{row: p.pool.QueryRow(ctx, query, args...)}
}

func (p *Pool) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }
func (p *Pool) Close()                         { p.pool.Close() }

type transaction struct{ tx pgx.Tx }

func (t transaction) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	tag, err := t.tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (t transaction) Query(ctx context.Context, query string, args ...any) (contracts.Rows, error) {
	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (t transaction) QueryRow(ctx context.Context, query string, args ...any) contracts.Row {
	return row{row: t.tx.QueryRow(ctx, query, args...)}
}

func (t transaction) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t transaction) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

type row struct{ row pgx.Row }

func (r row) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.ErrNoRows
	}
	return err
}
