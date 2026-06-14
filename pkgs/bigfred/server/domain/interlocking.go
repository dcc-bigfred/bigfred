package domain

import "time"

// Interlocking (Polish: nastawnia) is a logical signal box. At most
// one signalman occupies a given interlocking at a time (enforced by
// InterlockingSession in a later milestone). Location holds a free-text
// description of where the box sits on the layout ("opis" in the UI).
type Interlocking struct {
	ID        uint
	Name      string
	Location  string
	CreatedAt time.Time
}

// Table tells REL which physical table backs this struct.
func (Interlocking) Table() string { return "interlockings" }
