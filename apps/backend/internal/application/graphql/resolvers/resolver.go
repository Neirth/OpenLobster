package resolvers

import (
	"context"

	"github.com/neirth/openlobster/internal/domain/events"
)

// Resolver delegates to Deps (AgentRegistry + services) and event subscriptions.
type Resolver struct {
	Deps *Deps
	Sub  EventSubscriptionPort
}

// EventSubscriptionPort allows subscribing to domain events.
type EventSubscriptionPort interface {
	Subscribe(ctx context.Context, eventType string) (<-chan events.Event, error)
}

func NewResolver(deps *Deps) *Resolver {
	return &Resolver{Deps: deps}
}

// SetEventSubscription injects the subscription port (wired from main).
func (r *Resolver) SetEventSubscription(s EventSubscriptionPort) { r.Sub = s }
