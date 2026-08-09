package httpapi

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
)

// identityCtxKey is the unexported context key used by the auth
// middleware to attach the calling user's Identity to the request
// context. Handlers retrieve it via IdentityFromContext.
type identityCtxKey struct{}

type actorCtxKey struct{}

// WithIdentity returns a derived context that carries id. Used by the
// auth middleware exclusively.
func WithIdentity(ctx context.Context, id cmd.Identity) context.Context {
	return context.WithValue(ctx, identityCtxKey{}, id)
}

// IdentityFromContext returns the authenticated user's identity, or
// (zero, false) if the request is anonymous. Under impersonation this
// is the **subject**.
func IdentityFromContext(ctx context.Context) (cmd.Identity, bool) {
	id, ok := ctx.Value(identityCtxKey{}).(cmd.Identity)
	return id, ok
}

// WithActor stores the real caller when impersonating.
func WithActor(ctx context.Context, id cmd.Identity) context.Context {
	return context.WithValue(ctx, actorCtxKey{}, id)
}

// ActorFromContext returns the real caller when impersonation is active.
func ActorFromContext(ctx context.Context) (cmd.Identity, bool) {
	id, ok := ctx.Value(actorCtxKey{}).(cmd.Identity)
	return id, ok
}
