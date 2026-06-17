package errors

import "errors"

const (
	CodeVehicleTemplateNotFound     = "vehicle_template_not_found"
	CodeVehicleTemplateNameRequired = "vehicle_template_name_required"
	CodeVehicleTemplateNameTaken    = "vehicle_template_name_taken"
)

var (
	ErrVehicleTemplateNotFound     = errors.New(CodeVehicleTemplateNotFound)
	ErrVehicleTemplateNameRequired = errors.New(CodeVehicleTemplateNameRequired)
	ErrVehicleTemplateNameTaken    = errors.New(CodeVehicleTemplateNameTaken)
)
