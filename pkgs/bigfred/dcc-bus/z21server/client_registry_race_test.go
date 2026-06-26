package z21server

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

func TestRegistryConcurrentMutationsAndSnapshot(t *testing.T) {
	reg := NewRegistry()
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 21105}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, c := range reg.Snapshot() {
					_ = c.Paired
					_ = c.SubscribedLocos
					_ = c.BroadcastFlags
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			now := time.Now().UTC()
			c := reg.Touch(addr, now, false)
			reg.SubscribeLoco(c.Key, uint16(i%100)+1)
			reg.SetBroadcastFlags(c.Key, uint32(i))
			reg.SetLastActiveLoco(c.Key, uint16(i%50)+1)
			reg.BufferPairingCV(c.Key, 2, 111+i%10)
			reg.SetVirtualCV(c.Key, 1, 2, byte(i))
			reg.SetIdleBraked(c.Key, i%2 == 0)
			reg.ClearIdleBraked(c.Key)
		}
		cancel()
	}()

	wg.Wait()
}
