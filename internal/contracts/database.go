// Package contracts defines infrastructure ports shared by the API and
// persistence implementations.
package contracts

import (
	"context"
	"errors"
)

// ErrNoRows is returned by QueryRow().Scan() when a query found no record.
// Concrete database adapters should wrap or translate their driver sentinel.
var ErrNoRows = errors.New("no rows")

// Database is the small SQL surface needed by explicit-query repositories.
// A pgx/v5 adapter should implement it; result values intentionally avoid
// leaking a particular driver into application and handler packages.
type Database interface {
	BeginTx(context.Context) (Tx, error)
	Exec(context.Context, string, ...any) (rowsAffected int64, err error)
	Query(context.Context, string, ...any) (Rows, error)
	QueryRow(context.Context, string, ...any) Row
	Ping(context.Context) error
	Close()
}

type Tx interface {
	Exec(context.Context, string, ...any) (rowsAffected int64, err error)
	Query(context.Context, string, ...any) (Rows, error)
	QueryRow(context.Context, string, ...any) Row
	Commit(context.Context) error
	Rollback(context.Context) error
}

type Rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}

type Row interface {
	Scan(...any) error
}
