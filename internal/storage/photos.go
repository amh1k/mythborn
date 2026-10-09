// Package storage defines the private photo object-store boundary.
package storage

import (
	"context"
	"io"
	"time"
)

type PhotoStorage interface {
	Put(context.Context, string, string, io.Reader) error
	Open(context.Context, string) (io.ReadCloser, error)
	SignedURL(context.Context, string, time.Duration) (string, error)
	Delete(context.Context, string) error
}
