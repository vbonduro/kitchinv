package photostore

import (
	"context"
	"io"
)

type PhotoStore interface {
	Save(ctx context.Context, prefix, mimeType string, r io.Reader) (storageKey string, err error)
	Get(ctx context.Context, storageKey string) (io.ReadCloser, string, error)
	Delete(ctx context.Context, storageKey string) error
}
