// Copyright (c) OpenLobster contributors. See LICENSE for details.

package ports

import "context"

// TaskDispatcherPort is the domain boundary through which the Scheduler
// dispatches a task prompt for execution.
//
// Implementations live in the application layer and are responsible for
// routing the prompt through the full agentic pipeline (context building,
// LLM call, tool execution). This keeps the domain scheduler free of any
// dependency on application-level concerns such as session management or
// messaging adapters.
type TaskDispatcherPort interface {
	// Dispatch sends prompt through the agentic loop and returns any
	// execution error. Implementations must be safe for concurrent use
	// because the scheduler may call Dispatch from multiple goroutines.
	Dispatch(ctx context.Context, prompt string) error
}
