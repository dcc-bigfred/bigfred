package cli

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/keskad/loco/pkgs/loco/app"
	"github.com/spf13/cobra"
)

func NewProgCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "prog",
		Short: "Programming-track operations on the decoder",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewProgVolumeCommand(app))
	command.AddCommand(NewProgDetectDecoderCommand(app))
	return command
}

func NewProgVolumeCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "volume",
		Short: "Get or set decoder master volume (in percent)",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewProgVolumeSetCommand(app))
	command.AddCommand(NewProgVolumeGetCommand(app))
	return command
}

func NewProgVolumeSetCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId  uint8
		Timeout uint16
	}

	cmdArgs := Args{}
	command := &cobra.Command{
		Use:   "set PERCENT",
		Short: "Set decoder master volume in percent (0-100)",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			percent64, err := strconv.ParseUint(args[0], 10, 8)
			if err != nil {
				return fmt.Errorf("invalid volume value %q: %w", args[0], err)
			}
			percent := uint8(percent64)
			if percent > 100 {
				return fmt.Errorf("volume must be between 0 and 100 percent, got %d", percent)
			}

			return app.SetVolumeAction(cmdArgs.LocoId, percent, time.Second*time.Duration(cmdArgs.Timeout))
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Locomotive address on the programming track")

	return command
}

func NewProgVolumeGetCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId  uint8
		Timeout uint16
	}

	cmdArgs := Args{}
	command := &cobra.Command{
		Use:   "get",
		Short: "Get decoder master volume in percent",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			return app.GetVolumeAction(cmdArgs.LocoId, time.Second*time.Duration(cmdArgs.Timeout))
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Locomotive address on the programming track")

	return command
}

func NewProgDetectDecoderCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId  uint8
		Timeout uint16
	}

	cmdArgs := Args{}
	command := &cobra.Command{
		Use:   "detect-decoder",
		Short: "Identify the decoder on the programming track (CV7, CV8)",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			return app.DetectDecoderAction(cmdArgs.LocoId, time.Second*time.Duration(cmdArgs.Timeout))
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Locomotive address on the programming track")

	return command
}
