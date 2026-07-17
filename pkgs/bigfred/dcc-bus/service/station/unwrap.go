package station

import "github.com/keskad/loco/pkgs/loco/commandstation"

// innerStation is implemented by decorators that wrap a driver.
type innerStation interface {
	Inner() commandstation.Station
}

// AsStateObserver returns the push observer behind optional wrappers.
func AsStateObserver(s commandstation.Station) (commandstation.StateObserver, bool) {
	for s != nil {
		if obs, ok := s.(commandstation.StateObserver); ok {
			return obs, true
		}
		u, ok := s.(innerStation)
		if !ok {
			return nil, false
		}
		s = u.Inner()
	}
	return nil, false
}

// AsLocoInfoSubscriber returns the Z21 subscription helper behind
// optional wrappers.
func AsLocoInfoSubscriber(s commandstation.Station) (commandstation.LocoInfoSubscriber, bool) {
	for s != nil {
		if sub, ok := s.(commandstation.LocoInfoSubscriber); ok {
			return sub, true
		}
		u, ok := s.(innerStation)
		if !ok {
			return nil, false
		}
		s = u.Inner()
	}
	return nil, false
}

// AsSlotManager returns the SlotManager behind optional wrappers
func AsSlotManager(s commandstation.Station) (commandstation.SlotManager, bool) {
	for s != nil {
		if sm, ok := s.(commandstation.SlotManager); ok {
			return sm, true
		}
		u, ok := s.(innerStation)
		if !ok {
			return nil, false
		}
		s = u.Inner()
	}
	return nil, false
}

// AsBootSlotReconciler returns the boot reconciler behind optional wrappers.
func AsBootSlotReconciler(s commandstation.Station) (commandstation.BootSlotReconciler, bool) {
	for s != nil {
		if br, ok := s.(commandstation.BootSlotReconciler); ok {
			return br, true
		}
		u, ok := s.(innerStation)
		if !ok {
			return nil, false
		}
		s = u.Inner()
	}
	return nil, false
}

// AsSlotReconciler returns the SlotReconciler behind optional wrappers.
func AsSlotReconciler(s commandstation.Station) (commandstation.SlotReconciler, bool) {
	for s != nil {
		if sr, ok := s.(commandstation.SlotReconciler); ok {
			return sr, true
		}
		u, ok := s.(innerStation)
		if !ok {
			return nil, false
		}
		s = u.Inner()
	}
	return nil, false
}

// AsSlotObservable returns the SlotObservable behind optional wrappers so the
// leaser can subscribe to IN_USE/release events from the driver.
func AsSlotObservable(s commandstation.Station) (commandstation.SlotObservable, bool) {
	for s != nil {
		if obs, ok := s.(commandstation.SlotObservable); ok {
			return obs, true
		}
		u, ok := s.(innerStation)
		if !ok {
			return nil, false
		}
		s = u.Inner()
	}
	return nil, false
}

// AsPhysicalSlotAllocator returns the PhysicalSlotAllocator behind optional
// wrappers so the daemon can configure exclusive vs piggyback slot mode.
func AsPhysicalSlotAllocator(s commandstation.Station) (commandstation.PhysicalSlotAllocator, bool) {
	for s != nil {
		if psa, ok := s.(commandstation.PhysicalSlotAllocator); ok {
			return psa, true
		}
		u, ok := s.(innerStation)
		if !ok {
			return nil, false
		}
		s = u.Inner()
	}
	return nil, false
}

// AsSlotStealer returns the SlotStealer behind optional wrappers so the
// throttle can explicitly claim a foreign IN_USE slot.
func AsSlotStealer(s commandstation.Station) (commandstation.SlotStealer, bool) {
	for s != nil {
		if ss, ok := s.(commandstation.SlotStealer); ok {
			return ss, true
		}
		u, ok := s.(innerStation)
		if !ok {
			return nil, false
		}
		s = u.Inner()
	}
	return nil, false
}

// AsMetricsSource returns the MetricsSource behind optional wrappers, so the
// telemetry layer can read driver counters even when the station is decorated.
func AsMetricsSource(s commandstation.Station) (commandstation.MetricsSource, bool) {
	for s != nil {
		if ms, ok := s.(commandstation.MetricsSource); ok {
			return ms, true
		}
		u, ok := s.(innerStation)
		if !ok {
			return nil, false
		}
		s = u.Inner()
	}
	return nil, false
}

// AsZ21MetricsSource returns the Z21MetricsSource behind optional wrappers.
func AsZ21MetricsSource(s commandstation.Station) (commandstation.Z21MetricsSource, bool) {
	for s != nil {
		if ms, ok := s.(commandstation.Z21MetricsSource); ok {
			return ms, true
		}
		u, ok := s.(innerStation)
		if !ok {
			return nil, false
		}
		s = u.Inner()
	}
	return nil, false
}
