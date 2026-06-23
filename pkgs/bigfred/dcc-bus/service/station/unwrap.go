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
