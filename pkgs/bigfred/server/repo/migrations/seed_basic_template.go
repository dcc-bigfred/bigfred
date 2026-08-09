package migrations

import (
	"github.com/go-rel/rel"
)

const basicTemplateName = "Basic"

// basicFunctions is a generic F0–F10 catalogue for party/kiosk loco setup.
// F3 and F4 are forced momentary (1s) via seedTemplateFunctionsWithForce.
var basicFunctions = []templateFunctionSeed{
	{0, "F0", "light"},
	{1, "F1", "unspecified"},
	{2, "F2", "unspecified"},
	{3, "F3", "unspecified"},
	{4, "F4", "unspecified"},
	{5, "F5", "unspecified"},
	{6, "F6", "unspecified"},
	{7, "F7", "unspecified"},
	{8, "F8", "unspecified"},
	{9, "F9", "unspecified"},
	{10, "F10", "unspecified"},
}

var basicForceMomentary = forceMomentaryOverride{
	3: 1000,
	4: 1000,
}

func seedBasicTemplateUp(s *rel.Schema) {
	seedTemplateFunctionsWithForce(s, basicTemplateName, basicFunctions, basicForceMomentary)
}

func seedBasicTemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, basicTemplateName)
}
