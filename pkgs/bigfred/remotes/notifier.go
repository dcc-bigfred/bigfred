package remotes

import (
	"context"
	"sync"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// LocoStateNotifier fans locomotive state updates to registered observers.
type LocoStateNotifier struct {
	mu   sync.RWMutex
	obs  []LocoStateObserver
}

// NewLocoStateNotifier returns an empty observer registry.
func NewLocoStateNotifier() *LocoStateNotifier {
	return &LocoStateNotifier{}
}

// Register adds an observer. The same observer is not registered twice.
func (n *LocoStateNotifier) Register(obs LocoStateObserver) {
	if n == nil || obs == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, existing := range n.obs {
		if existing == obs {
			return
		}
	}
	n.obs = append(n.obs, obs)
}

// Notify delivers one snapshot to every registered observer.
func (n *LocoStateNotifier) Notify(ctx context.Context, snap contract.LocoStateWire) {
	if n == nil {
		return
	}
	for _, obs := range n.snapshot() {
		obs.OnLocoStateChanged(ctx, snap)
	}
}

func (n *LocoStateNotifier) snapshot() []LocoStateObserver {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]LocoStateObserver, len(n.obs))
	copy(out, n.obs)
	return out
}
