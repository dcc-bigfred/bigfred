package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/loco/app"
	"github.com/keskad/loco/pkgs/loco/decoders"
	"github.com/spf13/cobra"
)

func NewProgMappingCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "mapping",
		Short: "Program function-to-output mapping on the decoder",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewProgMappingSetCommand(app))
	return command
}

func NewProgMappingSetCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId   uint8
		Timeout  uint16
		Forward  bool
		Reverse  bool
	}

	cmdArgs := Args{}
	command := &cobra.Command{
		Use:   "set FUNCTION=OUTPUTS [FUNCTION=OUTPUTS ...]",
		Short: "Map function keys to one or more outputs",
		Long: `Map DCC function keys to physical outputs on the decoder.

Each assignment is FUNCTION=OUTPUTS; outputs are comma-separated:
  O<n> / FO<n> / AUX<n>   numbered output
  F0_F                    F0 forward headlight output
  F0_R                    F0 reverse headlight output

On RailBOX RB 2112/2110 output 1 is split into F0_F and F0_R; use those
tokens instead of O1.

Examples:
  loco prog mapping set F0=O1,O2 F1=O4,O6
  loco prog mapping set F0=F0_F --forward
  loco prog mapping set F0=F0_R --reverse
  loco prog mapping set F2=AUX3,AUX5 -l 3`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			assignments, err := decoders.ParseMappingAssignments(args)
			if err != nil {
				return err
			}

			direction, err := parseMappingDirection(cmdArgs.Forward, cmdArgs.Reverse)
			if err != nil {
				return err
			}

			results, err := app.SetMappingAction(
				cmdArgs.LocoId,
				assignments,
				direction,
				time.Second*time.Duration(cmdArgs.Timeout),
			)
			if err != nil {
				return err
			}

			for _, result := range results {
				printFunctionMappingResult(result)
			}
			return nil
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)
	command.Flags().BoolVar(&cmdArgs.Forward, "forward", false, "Map outputs for forward direction only")
	command.Flags().BoolVar(&cmdArgs.Reverse, "reverse", false, "Map outputs for reverse direction only")

	return command
}

func parseMappingDirection(forward, reverse bool) (decoders.MappingDirection, error) {
	switch {
	case forward && reverse:
		return decoders.MappingBoth, fmt.Errorf("use only one of --forward or --reverse")
	case forward:
		return decoders.MappingForward, nil
	case reverse:
		return decoders.MappingReverse, nil
	default:
		return decoders.MappingBoth, nil
	}
}

func printFunctionMappingResult(result app.FunctionMappingResult) {
	fmt.Printf("function=F%d\n", result.Function)
	fmt.Printf("outputs=%s\n", strings.Join(result.Outputs, ","))
	fmt.Printf("direction=%s\n", result.Direction)
	for _, write := range result.Writes {
		fmt.Printf("cv%d=%d\n", write.CV, write.Value)
	}
}
