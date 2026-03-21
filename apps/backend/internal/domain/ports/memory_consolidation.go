// Copyright (c) OpenLobster contributors. See LICENSE for details.

package ports

import "context"

// MemoryConsolidationPort defines the interface for the message-fragment-based
// memory consolidation pipeline.
type MemoryConsolidationPort interface {
	// Consolidate finds all unvalidated message fragments and runs them through
	// the plain-text Map-Reduce consolidation process.
	Consolidate(ctx context.Context) error
}
