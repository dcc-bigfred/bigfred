// Package helpers holds small, stateless utility functions shared
// across the server layer. Anything with dependencies or lifecycle
// belongs in a service struct instead.
package helpers

// MergeUserIDs returns a de-duplicated slice with `primary` first,
// followed by every non-zero `extra` id in input order. It is used to
// fold a vehicle owner together with active controllers (lessees,
// takeover holders, …) into the flat controllerUserIds set. A zero
// primary is dropped; an entirely empty input yields nil.
func MergeUserIDs(primary uint, extra ...uint) []uint {
	if primary == 0 && len(extra) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, 1+len(extra))
	out := make([]uint, 0, 1+len(extra))
	if primary != 0 {
		seen[primary] = struct{}{}
		out = append(out, primary)
	}
	for _, id := range extra {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
