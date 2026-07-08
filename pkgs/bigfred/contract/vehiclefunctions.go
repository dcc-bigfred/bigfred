package contract

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

const (
	// VehicleFunctionsKeyTmpl is the Redis STRING key — and pub/sub channel —
	// carrying resolved function catalogues for layout roster vehicles.
	VehicleFunctionsKeyTmpl = "bigfred:layout:%d:vehicle_functions"
)

// VehicleFunctionsKey is the Redis STRING key and pub/sub channel for
// per-layout vehicle function catalogues.
func VehicleFunctionsKey(layoutID uint) string {
	return fmt.Sprintf(VehicleFunctionsKeyTmpl, layoutID)
}

// VehicleFunctions lists resolved DCC function metadata for drivable roster
// vehicles on one layout.
type VehicleFunctions struct {
	LayoutID  uint                       `json:"layoutId"`
	UpdatedAt int64                      `json:"updatedAt"`
	Vehicles  []VehicleFunctionCatalogue `json:"vehicles"`
}

// VehicleFunctionCatalogue is the function list for one roster vehicle.
type VehicleFunctionCatalogue struct {
	VehicleID string               `json:"vehicleId"`
	Addr      uint16               `json:"addr"`
	Functions []FunctionDefinition `json:"functions"`
}

// FunctionDefinition is one F0–F31 slot from the vehicle catalogue.
type FunctionDefinition struct {
	Num        uint8  `json:"num"`
	Name       string `json:"name"`
	Icon       string `json:"icon"`
	Position   int    `json:"position"`
	Momentary  bool   `json:"momentary,omitempty"`
	DurationMs int    `json:"durationMs,omitempty"`
}

// GetMomentaryDuration returns the auto-off delay for a momentary function.
func (f FunctionDefinition) GetMomentaryDuration() time.Duration {
	return time.Duration(f.DurationMs) * time.Millisecond
}

// SortFunctionDefinitions orders slots by display position, then F number.
func SortFunctionDefinitions(fns []FunctionDefinition) {
	sort.Slice(fns, func(i, j int) bool {
		if fns[i].Position != fns[j].Position {
			return fns[i].Position < fns[j].Position
		}
		return fns[i].Num < fns[j].Num
	})
}

// FunctionLabel returns the WiThrottle label for one slot.
func (f FunctionDefinition) FunctionLabel() string {
	if f.Name != "" {
		return f.Name
	}
	if f.Icon != "" && f.Icon != "unspecified" {
		return f.Icon
	}
	return fmt.Sprintf("F%d", f.Num)
}

// VehicleFunctionsChangedAddrs returns DCC addresses whose function catalogues
// differ between prev and next snapshots.
func VehicleFunctionsChangedAddrs(prev, next VehicleFunctions) []uint16 {
	prevByAddr := make(map[uint16][]FunctionDefinition, len(prev.Vehicles))
	for _, v := range prev.Vehicles {
		prevByAddr[v.Addr] = v.Functions
	}
	changed := make([]uint16, 0)
	seen := make(map[uint16]struct{})
	for _, v := range next.Vehicles {
		if functionDefinitionsEqual(prevByAddr[v.Addr], v.Functions) {
			continue
		}
		if _, dup := seen[v.Addr]; dup {
			continue
		}
		seen[v.Addr] = struct{}{}
		changed = append(changed, v.Addr)
	}
	return changed
}

func functionDefinitionsEqual(a, b []FunctionDefinition) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// UnmarshalVehicleFunctions decodes a pub/sub payload or GET value.
func UnmarshalVehicleFunctions(raw []byte) (VehicleFunctions, error) {
	var snap VehicleFunctions
	if err := json.Unmarshal(raw, &snap); err != nil {
		return VehicleFunctions{}, err
	}
	return snap, nil
}

// BuildVehicleFunctionsPayload marshals the JSON SET and PUBLISHed under
// VehicleFunctionsKey(layoutID).
func BuildVehicleFunctionsPayload(layoutID uint, updatedAt int64, vehicles []VehicleFunctionCatalogue) ([]byte, error) {
	return Marshal(VehicleFunctions{LayoutID: layoutID, UpdatedAt: updatedAt, Vehicles: vehicles})
}
