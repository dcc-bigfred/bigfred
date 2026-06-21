package cli

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/keskad/loco/pkgs/loco/app"
	"github.com/spf13/cobra"
)

const progLocoFlagUsage = "Locomotive address; uses PoM when non-zero, programming track when 0"

func NewProgCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "prog",
		Short: "Programming-track operations on the decoder",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewCVCommand(app))
	command.AddCommand(NewProgVolumeCommand(app))
	command.AddCommand(NewProgBrightnessCommand(app))
	command.AddCommand(NewAddrCommand(app))
	command.AddCommand(NewProgFactoryResetCommand(app))
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
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)

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

			percent, err := app.GetVolumeAction(cmdArgs.LocoId, time.Second*time.Duration(cmdArgs.Timeout))
			if err != nil {
				return err
			}
			fmt.Printf("%d\n", percent)
			return nil
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)

	return command
}

func NewProgBrightnessCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "brightness",
		Short: "Get or set per-output lighting brightness (in percent)",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewProgBrightnessSetCommand(app))
	command.AddCommand(NewProgBrightnessGetCommand(app))
	command.AddCommand(NewProgBrightnessListCommand(app))
	command.AddCommand(NewProgBrightnessTestCommand(app))
	return command
}

func NewProgBrightnessSetCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId  uint8
		Value   uint8
		Timeout uint16
	}

	cmdArgs := Args{}
	command := &cobra.Command{
		Use:   "set OUTPUT",
		Short: "Set lighting brightness for an output (0-100 percent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			output64, err := strconv.ParseUint(args[0], 10, 8)
			if err != nil {
				return fmt.Errorf("invalid output number %q: %w", args[0], err)
			}
			if cmdArgs.Value > 100 {
				return fmt.Errorf("brightness must be between 0 and 100 percent, got %d", cmdArgs.Value)
			}

			return app.SetBrightnessAction(cmdArgs.LocoId, uint8(output64), cmdArgs.Value, time.Second*time.Duration(cmdArgs.Timeout))
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)
	command.Flags().Uint8VarP(&cmdArgs.Value, "value", "V", 0, "Brightness in percent (0-100)")

	command.MarkFlagRequired("value")

	return command
}

func NewProgBrightnessGetCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId  uint8
		Timeout uint16
	}

	cmdArgs := Args{}
	command := &cobra.Command{
		Use:   "get OUTPUT",
		Short: "Get lighting brightness for an output (in percent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			output64, err := strconv.ParseUint(args[0], 10, 8)
			if err != nil {
				return fmt.Errorf("invalid output number %q: %w", args[0], err)
			}

			percent, err := app.GetBrightnessAction(cmdArgs.LocoId, uint8(output64), time.Second*time.Duration(cmdArgs.Timeout))
			if err != nil {
				return err
			}
			fmt.Printf("%d\n", percent)
			return nil
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)

	return command
}

func NewProgBrightnessListCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId  uint8
		Timeout uint16
	}

	cmdArgs := Args{}
	command := &cobra.Command{
		Use:   "list",
		Short: "List brightness of all addressable outputs (in percent)",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			levels, err := app.ListBrightnessAction(cmdArgs.LocoId, time.Second*time.Duration(cmdArgs.Timeout))
			if err != nil {
				return err
			}
			printBrightnessLevels(levels)
			return nil
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)

	return command
}

func NewProgBrightnessTestCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId  uint8
		Timeout uint16
		Pause   uint16
	}

	cmdArgs := Args{Pause: 5}
	command := &cobra.Command{
		Use:   "test",
		Short: "Blink each output twice to identify lighting wiring",
		Long: `Save all output brightness CV values, blink each output twice
(0% -> 50%), then restore the original values.

Turn on all light functions on the locomotive before the test starts.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			fmt.Printf("Turn on all light functions on the locomotive before the test starts.\n")
			if cmdArgs.Pause > 0 {
				fmt.Printf("Starting in %d seconds…\n", cmdArgs.Pause)
				time.Sleep(time.Second * time.Duration(cmdArgs.Pause))
			}

			fmt.Printf("Running brightness test…\n")
			snapshot, err := app.TestBrightnessAction(
				cmdArgs.LocoId,
				time.Second*time.Duration(cmdArgs.Timeout),
			)
			if err != nil {
				return err
			}

			printBrightnessSnapshot(snapshot)
			fmt.Printf("Brightness test complete.\n")
			return nil
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)
	command.Flags().Uint16VarP(&cmdArgs.Pause, "pause", "", 5, "Seconds to wait after the reminder before starting the test")

	return command
}

func NewProgFactoryResetCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoId       uint8
		Timeout      uint16
		Settle       uint16
		Recovery     uint16
		Retries      uint8
		PreserveAddr bool
	}

	cmdArgs := Args{Settle: 300, Recovery: 2}
	command := &cobra.Command{
		Use:   "factory-reset",
		Short: "Reset the decoder to factory defaults (CV8)",
		Long: `Factory reset via CV8 write, decoder-specific:
  RailBOX RB23xx: CV8 = 1
  ESU LokSound 5: CV8 = 8
  ZIMO MS/MN:     CV8 = 8`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			result, err := app.FactoryResetAction(
				cmdArgs.LocoId,
				cmdArgs.PreserveAddr,
				time.Second*time.Duration(cmdArgs.Timeout),
				time.Millisecond*time.Duration(cmdArgs.Settle),
				time.Second*time.Duration(cmdArgs.Recovery),
				cmdArgs.Retries,
			)
			if err != nil {
				return err
			}
			printFactoryResetResult(result)
			return nil
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint16VarP(&cmdArgs.Settle, "settle", "", 300, "Time in milliseconds between CV writes")
	command.Flags().Uint16VarP(&cmdArgs.Recovery, "recovery", "", 2, "Seconds to wait after reset before restoring address")
	command.Flags().Uint8VarP(&cmdArgs.Retries, "retry", "", 0, "Retry CV reads before reset")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)
	command.Flags().BoolVar(&cmdArgs.PreserveAddr, "preserve-addr", false, "Restore the locomotive address after reset")

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
		Short: "Identify the decoder (CV7, CV8)",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			id, err := app.DetectDecoderAction(cmdArgs.LocoId, time.Second*time.Duration(cmdArgs.Timeout))
			if err != nil {
				return err
			}
			printDecoderIdentification(id)
			return nil
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)

	return command
}
