package validation

import (
	"strings"

	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const MaxVehicleTemplateNameLen = 64

// SanitiseVehicleTemplateName trims whitespace and enforces a non-empty name.
func SanitiseVehicleTemplateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", svcerrors.ErrVehicleTemplateNameRequired
	}
	if len(name) > MaxVehicleTemplateNameLen {
		name = name[:MaxVehicleTemplateNameLen]
	}
	return name, nil
}
