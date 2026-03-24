package secrets

import (
	"context"
	"errors"
)

// ErrNotFound is returned by SecretsProvider.Get when the requested key does
// not exist in the backend.  Callers should use errors.Is to distinguish a
// missing key from a genuine backend failure.
var ErrNotFound = errors.New("secret not found")

type SecretsProvider interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}
