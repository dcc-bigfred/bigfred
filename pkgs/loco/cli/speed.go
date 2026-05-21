package cli

import (
	"fmt"
	"strconv"

	"github.com/keskad/loco/pkgs/loco/app"
	"github.com/spf13/cobra"
)

func NewSpeedCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "speed",
		Short: "Get or set the speed and direction of a locomotive",
		RunE: func(command *cobra.Command, args []string) error {
			return command.Help()
		},
	}

	command.AddCommand(NewSpeedSetCommand(app))
	command.AddCommand(NewSpeedGetCommand(app))

	return command
}

func NewSpeedSetCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId     uint8
		Forward    bool
		SpeedSteps uint8
		Timeout    uint16
	}

	cmdArgs := Args{SpeedSteps: 128} // Default to 128 speed steps
	command := &cobra.Command{
		Use:   "set SPEED",
		Short: "Set the speed and direction of a locomotive",
		Long: `Set the speed and direction of a locomotive.

SPEED should be a value from 0 to the maximum for your speed steps:
  - For 14 speed steps: 0-15 (0=stop, 1=emergency stop, 2-15=steps 1-14)
  - For 28 speed steps: 0-28 (0=stop, 1=emergency stop, 2-28=steps 1-27)
  - For 128 speed steps: 0-127 (0=stop, 1=emergency stop, 2-127=steps 1-126)

Examples:
  loco speed set 50 --loco 3 --forward
  loco speed set 0 --loco 3                    # Stop locomotive
  loco speed set 30 --loco 5 --steps 28        # Set speed using 28 speed steps
  loco speed set 1 --loco 3                    # Emergency stop`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			// Parse speed value
			speed64, err := strconv.ParseUint(args[0], 10, 8)
			if err != nil {
				return fmt.Errorf("invalid speed value %q: %w", args[0], err)
			}
			speed := uint8(speed64)

			// Validate speed based on speed steps
			var maxSpeed uint8
			switch cmdArgs.SpeedSteps {
			case 14:
				maxSpeed = 15
			case 28:
				maxSpeed = 28
			case 128:
				maxSpeed = 127
			default:
				return fmt.Errorf("invalid speed steps %d (must be 14, 28, or 128)", cmdArgs.SpeedSteps)
			}

			if speed > maxSpeed {
				return fmt.Errorf("speed %d exceeds maximum %d for %d speed steps", speed, maxSpeed, cmdArgs.SpeedSteps)
			}

			return app.SetSpeedAction(cmdArgs.LocoId, speed, cmdArgs.Forward, cmdArgs.SpeedSteps)
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Locomotive address (required)")
	command.Flags().BoolVarP(&cmdArgs.Forward, "forward", "f", false, "Set direction to forward (default is reverse)")
	command.Flags().Uint8VarP(&cmdArgs.SpeedSteps, "steps", "s", 128, "Speed steps: 14, 28, or 128 (default: 128)")

	command.MarkFlagRequired("loco")

	return command
}

func NewSpeedGetCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId  uint8
		Timeout uint16
	}

	cmdArgs := Args{}
	command := &cobra.Command{
		Use:   "get",
		Short: "Get the current speed and direction of a locomotive",
		Long: `Get the current speed and direction of a locomotive.

Examples:
  loco speed get --loco 3
  loco speed get -l 5`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			speed, forward, err := app.GetSpeedAction(cmdArgs.LocoId)
			if err != nil {
				return err
			}

			direction := "reverse"
			if forward {
				direction = "forward"
			}

			fmt.Printf("Locomotive %d: speed=%d direction=%s\n", cmdArgs.LocoId, speed, direction)
			return nil
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Locomotive address (required)")

	command.MarkFlagRequired("loco")

	return command
}
