// Copyright (c) OpenLobster contributors. See LICENSE for details.

package ports

import "context"

// SecretsProvider abstracts secret storage/retrieval for MCP OAuth and other
// subsystems. Implementations are expected to return ErrNotFound from Get when
// a key does not exist.
type SecretsProvider interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}
