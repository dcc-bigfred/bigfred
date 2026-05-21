package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/keskad/loco/pkgs/loco/app"
	"github.com/spf13/cobra"
)

func NewFnCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "fn",
		Short: "Control locomotive functions using a command station",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	// Add the list subcommand
	command.AddCommand(NewFnListCommand(app))
	command.AddCommand(NewFnSetCommand(app))

	return command
}

func NewFnSetCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId  uint8
		Track   string
		Timeout uint16
		Off     bool
	}

	cmdArgs := Args{}
	command := &cobra.Command{
		Use:   "set",
		Short: "Sends a function request to the decoder",
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			// mode selection and validation
			track, trackErr := TrackOrDefault(cmdArgs.Track, cmdArgs.LocoId)
			if trackErr != nil {
				return trackErr
			}
			if len(args) == 0 {
				return errors.New("need to specify a function number")
			}

			fnNum64, err := strconv.ParseUint(args[0], 10, 8)
			if err != nil {
				return fmt.Errorf("invalid function number %q: %w", args[0], err)
			}

			return app.SendFnAction(track, cmdArgs.LocoId, int(fnNum64), !cmdArgs.Off)
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().BoolVarP(&cmdArgs.Off, "off", "d", false, "Toggle the function off")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Use locomotive under specific address")
	command.Flags().StringVarP(&cmdArgs.Track, "track", "t", "", "Track type: 'pom' for programming on main, 'prog' for programming track, or empty for automatic selection")

	return command
}

func NewFnListCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId  uint8
		Timeout uint16
	}

	cmdArgs := Args{}
	command := &cobra.Command{
		Use:   "list",
		Short: "Lists all active functions on the locomotive",
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			return app.ListFnAction(cmdArgs.LocoId)
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Use locomotive under specific address")

	return command
}
