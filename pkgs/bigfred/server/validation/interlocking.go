package validation

import (
	"strings"

	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const maxInterlockingNameLen = 64
const maxInterlockingLocationLen = 512

func SanitiseInterlockingName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxInterlockingNameLen {
		return "", svcerrors.ErrInterlockingNameRequired
	}
	return name, nil
}

func SanitiseInterlockingLocation(location string) string {
	location = strings.TrimSpace(location)
	if len(location) > maxInterlockingLocationLen {
		return location[:maxInterlockingLocationLen]
	}
	return location
}
