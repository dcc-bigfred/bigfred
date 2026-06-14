// Package security holds the stateless policy layer described in §7a.3.
//
// Every "is the actor allowed to do X to Y?" decision lives here as a
// pure function on already-loaded domain objects, returning a Decision.
// Policies never touch the database, HTTP or context — those concerns
// belong to the services and middleware that call them.
package security

// Decision is the single return type of every security check. Reason
// is intentionally machine-readable so the HTTP layer can map it to a
// status code and the UI can localise it.
type Decision struct {
	Allowed bool
	Reason  string
}

// Allow is the canonical positive Decision. Re-used to avoid allocation
// on the hot path.
var Allow = Decision{Allowed: true}

// Deny constructs a negative Decision with a machine-readable reason.
func Deny(reason string) Decision {
	return Decision{Allowed: false, Reason: reason}
}
