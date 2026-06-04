package domain

import "time"

// FunctionIcon is a closed catalogue (§3a.8). Wire values are slug strings.
type FunctionIcon string

// DccFunction is one F0–F31 slot in the unified dcc_functions table.
// Exactly one of VehicleID or TemplateID is non-nil per row.
type DccFunction struct {
	ID         uint
	VehicleID  *uint `db:"vehicle_id"`
	TemplateID *uint `db:"template_id"`
	Num        uint8
	Name       string
	Icon       FunctionIcon
	Position   int
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// Table tells REL which physical table backs this struct.
func (DccFunction) Table() string { return "dcc_functions" }

// functionIconOrder is the authoritative catalogue order for
// GET /api/v1/function-icons.
var functionIconOrder = []FunctionIcon{
	"unspecified", "light", "engine", "sound", "horn", "coupler",
	"interior_light", "engine_room_light", "shunting_steps_light",
	"inspection_light", "cab_light", "headlight", "roof_headlight",
	"red_lights", "vestibule_lights", "destination_board_lights",
	"door", "smoke", "speaker", "whistle", "toilet", "compressor",
	"brake_sound", "coal_shoveling", "fan", "hand_brake", "injector",
	"mute_sounds", "radio_command", "shunting_mode", "valve", "wheels",
	"wipers", "sander", "long_whistle", "short_whistle", "pantograph",
	"volume_up", "volume_down", "heavy_load", "wifi", "pc2_signal",
	"coupling", "uncoupling", "oil_pump", "brake_sound_mute",
	"wheel_squeal", "bell", "coal_bunker", "watering",
	"crane_up", "crane_down", "crane_left", "crane_right", "crane_hook",
	"sifa", "firebox", "steam_release", "window", "buffer", "danger",
	"engineer_laugh", "stairs", "beacon_light", "side_lights",
	"turn_signal_left", "turn_signal_right",
}

var validFunctionIcons map[FunctionIcon]struct{}

func init() {
	validFunctionIcons = make(map[FunctionIcon]struct{}, len(functionIconOrder))
	for _, icon := range functionIconOrder {
		validFunctionIcons[icon] = struct{}{}
	}
}

// FunctionIcons returns the closed catalogue in display order.
func FunctionIcons() []FunctionIcon {
	out := make([]FunctionIcon, len(functionIconOrder))
	copy(out, functionIconOrder)
	return out
}

// IsValid reports whether icon is in the catalogue.
func (i FunctionIcon) IsValid() bool {
	_, ok := validFunctionIcons[i]
	return ok
}

// ValidFunctionNum reports whether n is in F0–F31.
func ValidFunctionNum(n uint8) bool { return n <= 31 }
