package slotlease

// ReleaseReason labels slot_released_total{reason}.
type ReleaseReason string

const (
	ReleaseSessionClose      ReleaseReason = "session_close"
	ReleaseIdleTimeout       ReleaseReason = "idle_timeout"
	ReleaseCapEvict          ReleaseReason = "cap_evict"
	ReleaseRosterRetire      ReleaseReason = "roster_retire"
	ReleaseTrainDissolved    ReleaseReason = "train_dissolved"
	ReleasePermissionRevoked ReleaseReason = "permission_revoked"
	ReleaseSwitcherChange    ReleaseReason = "switcher_change"
	ReleaseShutdown          ReleaseReason = "shutdown"
	ReleaseBootReconcile     ReleaseReason = "boot_reconcile"
	ReleasePendingRetry      ReleaseReason = "release_pending_retry"
	ReleaseGraceEvict        ReleaseReason = "grace_evict" // D20: budget pressure
)
