package resolvers

import (
	"context"

	"github.com/neirth/openlobster/internal/domain/events"
)

// Resolver delega a Deps (AgentRegistry + servicios) y suscripciones de eventos.
type Resolver struct {
	Deps *Deps
	Sub  EventSubscriptionPort
}

// EventSubscriptionPort permite suscribirse a eventos del dominio.
type EventSubscriptionPort interface {
	Subscribe(ctx context.Context, eventType string) (<-chan events.Event, error)
}

func NewResolver(deps *Deps) *Resolver {
	return &Resolver{Deps: deps}
}

// SetEventSubscription inyecta el puerto de suscripción (por main).
func (r *Resolver) SetEventSubscription(s EventSubscriptionPort) { r.Sub = s }
