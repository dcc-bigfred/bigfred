package errors

import "errors"

const (
	CodeLayoutVehicleAlreadyOnRoster = "layout_vehicle_already_on_roster"
	CodeLayoutVehicleNotOnRoster     = "layout_vehicle_not_on_roster"
	CodeLayoutTrainAlreadyOnRoster   = "layout_train_already_on_roster"
	CodeLayoutTrainNotOnRoster       = "layout_train_not_on_roster"
)

var (
	ErrLayoutVehicleAlreadyOnRoster = errors.New(CodeLayoutVehicleAlreadyOnRoster)
	ErrLayoutVehicleNotOnRoster     = errors.New(CodeLayoutVehicleNotOnRoster)
	ErrLayoutTrainAlreadyOnRoster   = errors.New(CodeLayoutTrainAlreadyOnRoster)
	ErrLayoutTrainNotOnRoster       = errors.New(CodeLayoutTrainNotOnRoster)
)
