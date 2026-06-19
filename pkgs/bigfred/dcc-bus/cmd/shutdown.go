package cmd

// Shutdown releases all LocoNet slots and closes the command station
// driver cleanly. It must be called before the process exits so that
// the command station can reclaim slots and physical throttles (FREDs)
// can take immediate control of any previously controlled locomotives.
//
// The LocoNet driver sends OPC_SLOT_STAT1 COMMON for every cached slot
// before closing the transport connection.
func (r *Router) Shutdown() {
	if err := r.station.CleanUp(); err != nil {
		r.log.WithError(err).Warn("dcc-bus shutdown: station cleanup failed")
	}
}
