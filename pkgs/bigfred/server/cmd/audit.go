package cmd

import "context"

// AuditActor carries the identity of the user responsible for an audited
// action. Both fields are denormalized at write time so historical entries
// remain readable even after the user's login is renamed or the account is
// deleted.
type AuditActor struct {
	UserID uint
	Login  string
}

// AuditPublisher is the narrow interface implemented by
// service.AuditService. Passing nil is always safe — every call site
// checks for a nil receiver so audit is a best-effort, non-blocking
// concern.
type AuditPublisher interface {
	// Publish appends one entry to the audit stream. layoutID == 0 means
	// the event is not scoped to a specific layout (e.g. user CRUD).
	Publish(ctx context.Context, layoutID uint, actor AuditActor, msg string, vars map[string]string) error
}
