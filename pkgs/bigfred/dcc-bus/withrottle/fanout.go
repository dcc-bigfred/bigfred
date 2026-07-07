package withrottle

import (
	"context"
	"fmt"
	"sort"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

var _ remotes.LocoStateObserver = (*Server)(nil)

// OnLocoStateChanged pushes M…A lines to paired clients subscribed to the
// address. The commanding handset (originClientKey) is skipped because the
// adapter already echoed the state directly; empty origin means a non-handset
// source, so nobody is skipped.
func (s *Server) OnLocoStateChanged(ctx context.Context, snap contract.LocoStateWire, originClientKey string) {
	_ = ctx
	if s.registry == nil {
		return
	}
	for _, key := range s.registry.Subscribers(snap.Address) {
		if key == originClientKey {
			continue
		}
		client, ok := s.registry.Get(key)
		if !ok || client.Session == nil {
			continue
		}
		throttleID, locoKey, ok := s.registry.findThrottleForAddr(key, snap.Address)
		if !ok {
			locoKey = locoKeyForAddr(snap.Address)
			throttleID = '0'
		}
		s.registry.setLastSpeed(key, throttleID, snap.Address, snap.Speed)
		lines := buildLocoNotify(throttleID, locoKey, snap, s.cfg.SpeedSteps)
		for _, line := range lines {
			_ = s.writeLine(key, line)
		}
	}
}

func buildLocoNotify(throttleID byte, locoKey string, snap contract.LocoStateWire, speedSteps uint) []string {
	id := string(throttleID)
	speed := wireSpeedFromDCC(snap.Speed, speedSteps)
	dir := 0
	if snap.Forward {
		dir = 1
	}
	lines := []string{
		fmt.Sprintf("M%sA%s%sV%d", id, locoKey, propSep, speed),
		fmt.Sprintf("M%sA%s%sR%d", id, locoKey, propSep, dir),
	}
	if len(snap.Functions) > 0 {
		fns := make([]int, 0, len(snap.Functions))
		for fn := range snap.Functions {
			if fn > maxWiThrottleFunction {
				continue
			}
			fns = append(fns, fn)
		}
		sort.Ints(fns)
		for _, fn := range fns {
			state := 0
			if snap.Functions[fn] {
				state = 1
			}
			lines = append(lines, fmt.Sprintf("M%sA%s%sF%d%d", id, locoKey, propSep, state, fn))
		}
	}
	return lines
}
