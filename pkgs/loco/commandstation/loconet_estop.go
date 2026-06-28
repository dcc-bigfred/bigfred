package commandstation

// EmergencyStopper is an OPTIONAL capability for drivers that can bypass the
// normal TX queue for an emergency stop so it is not delayed behind a full
// throttle queue.
type EmergencyStopper interface {
	EmergencyStop(addr LocoAddr, forward bool) error
}

// EmergencyStop sends wire speed 1 (DCC emergency stop) on the estop-priority
// channel, bypassing a full normal TX queue.
func (l *LocoNet) EmergencyStop(addr LocoAddr, forward bool) error {
	slot, err := l.acquireSlot(addr)
	if err != nil {
		return err
	}
	fl := l.addrFnLock(addr)
	fl.Lock()
	defer fl.Unlock()

	if err := l.txEnqueue(lnBuildSetSpeed(slot, 1), lnTxPriorityEstop); err != nil {
		return err
	}
	l.setSpd(addr, 1)
	l.markAcquired(addr)

	dirf := l.getDirf(addr)
	want := dirf
	if forward {
		want |= 0x20
	} else {
		want &^= 0x20
	}
	if want != dirf {
		if err := l.txEnqueue(lnBuildSetDirF(slot, want), lnTxPriorityEstop); err != nil {
			return err
		}
		l.setDirf(addr, want)
	}
	return nil
}
