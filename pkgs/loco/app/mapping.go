package app

import (
	"time"

	"github.com/keskad/loco/pkgs/loco/decoders"
)

func mappingDirectionLabel(direction decoders.MappingDirection) string {
	switch direction {
	case decoders.MappingForward:
		return "forward"
	case decoders.MappingReverse:
		return "reverse"
	default:
		return "both"
	}
}

func (app *LocoApp) SetMappingAction(locoId uint8, assignments []decoders.MappingAssignment, direction decoders.MappingDirection, timeout time.Duration) ([]FunctionMappingResult, error) {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return nil, cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)
	decoder, err := decoders.DetectMapping(cv)
	if err != nil {
		return nil, err
	}

	results := make([]FunctionMappingResult, 0, len(assignments))
	for _, assignment := range assignments {
		writes, err := decoder.SetFunctionMapping(assignment.Function, assignment.Outputs, direction)
		if err != nil {
			return results, err
		}

		labels := make([]string, len(assignment.Outputs))
		for i, output := range assignment.Outputs {
			labels[i] = output.Label
		}

		results = append(results, FunctionMappingResult{
			Function:  assignment.Function,
			Outputs:   labels,
			Direction: mappingDirectionLabel(direction),
			Writes:    writes,
		})
	}
	return results, nil
}
