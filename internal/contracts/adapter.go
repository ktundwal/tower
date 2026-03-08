package contracts

import "context"

// SessionAdapter is the in-process v1 boundary between Tower core and an adapter.
// Managed launch lives in the runtime/adapter-specific boundary underneath it.
type SessionAdapter interface {
	Discover(ctx context.Context) ([]SessionDescriptor, error)
	Subscribe(ctx context.Context) (<-chan Event, error)
	Snapshot(ctx context.Context, sessionID SessionID) (SessionSnapshot, error)
	Capabilities(ctx context.Context, sessionID SessionID) (CapabilitySet, error)
	Perform(ctx context.Context, sessionID SessionID, action Action) (ActionResult, error)
}
