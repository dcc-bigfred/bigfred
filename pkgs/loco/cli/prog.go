package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/loco/app"
	"github.com/keskad/loco/pkgs/loco/decoders"
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
	command.AddCommand(NewProgMappingCommand(app))
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
		Use:   "set OUTPUT=PERCENT [OUTPUT=PERCENT ...]",
		Short: "Set lighting brightness for one or more outputs (0-100 percent)",
		Long: `Set lighting brightness per output, in percent (0-100).

Each assignment is OUTPUT=PERCENT; the output may be a bare number or use an
O/FO/AUX prefix. Assignments may be comma- or space-separated.

Examples:
  loco prog brightness set O1=50,O2=5
  loco prog brightness set O1=10 O6=50
  loco prog brightness set 3=20 -l 3

Legacy single-output form (uses --value):
  loco prog brightness set O3 --value=20`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			settings, err := brightnessSettingsFromArgs(args, command, cmdArgs.Value)
			if err != nil {
				return err
			}

			applied, err := app.SetBrightnessAction(cmdArgs.LocoId, settings, time.Second*time.Duration(cmdArgs.Timeout))
			if err != nil {
				return err
			}
			printBrightnessLevels(applied)
			return nil
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)
	command.Flags().Uint8VarP(&cmdArgs.Value, "value", "V", 0, "Brightness in percent for the legacy OUTPUT form (0-100)")

	return command
}

// brightnessSettingsFromArgs parses OUTPUT=PERCENT assignments. When no argument
// contains "=", it falls back to the legacy form where outputs take the --value flag.
func brightnessSettingsFromArgs(args []string, command *cobra.Command, value uint8) ([]decoders.BrightnessSetting, error) {
	for _, a := range args {
		if strings.Contains(a, "=") {
			return decoders.ParseBrightnessArgs(args)
		}
	}

	if !command.Flags().Changed("value") {
		return nil, fmt.Errorf("specify brightness as OUTPUT=PERCENT (e.g. O1=50), or pass --value for the legacy single-output form")
	}

	paired := make([]string, 0, len(args))
	for _, a := range args {
		fields := strings.FieldsFunc(a, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t'
		})
		for _, field := range fields {
			paired = append(paired, fmt.Sprintf("%s=%d", field, value))
		}
	}
	return decoders.ParseBrightnessArgs(paired)
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
		LocoId     uint8
		Timeout    uint16
		Pause      uint16
		Brightness uint8
	}

	cmdArgs := Args{Pause: 5, Brightness: decoders.BrightnessTestActivePercentDefault}
	command := &cobra.Command{
		Use:   "test",
		Short: "Identify which physical light is wired to each output",
		Long: `Interactive test to map physical lights to decoder outputs (O/FO/AUX).

Turn on all lighting functions on the vehicle first. The test turns all outputs
off, then lights one output at a time. Note which physical light is on,
then press Enter to continue to the next output. Use --brightness to set how
bright each output is during the test (default 50%%).

Use the output numbers when running: loco prog mapping set`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			fmt.Printf("Turn on all lighting functions on the vehicle before the test starts.\n")
			if cmdArgs.Pause > 0 {
				fmt.Printf("Starting in %d seconds…\n", cmdArgs.Pause)
				time.Sleep(time.Second * time.Duration(cmdArgs.Pause))
			}

			reader := bufio.NewReader(os.Stdin)
			snapshot, err := app.TestBrightnessAction(
				cmdArgs.LocoId,
				cmdArgs.Brightness,
				time.Second*time.Duration(cmdArgs.Timeout),
				decoders.BrightnessIdentifyHooks{
					OnOutput: func(state decoders.OutputBrightness, index, total int) error {
						fmt.Printf("\n--- Output O%d (%d/%d)  cv%d=%d ---\n",
							state.Output, index, total, state.CV, state.Value)
						fmt.Printf("Only this output is lit. Note which physical light is on.\n")
						return nil
					},
					WaitNext: func(state decoders.OutputBrightness, index, total int) error {
						if index < total {
							fmt.Printf("Press Enter for the next output… ")
						} else {
							fmt.Printf("Press Enter to finish… ")
						}
						if _, err := reader.ReadString('\n'); err != nil {
							return fmt.Errorf("failed to read input: %w", err)
						}
						return nil
					},
				},
			)
			if err != nil {
				return err
			}

			fmt.Printf("\nSaved brightness values (restored):\n")
			printBrightnessSnapshot(snapshot)
			fmt.Printf("Brightness identification complete.\n")
			return nil
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)
	command.Flags().Uint16VarP(&cmdArgs.Pause, "pause", "", 5, "Seconds to wait after the reminder before starting the test")
	command.Flags().Uint8VarP(&cmdArgs.Brightness, "brightness", "b", decoders.BrightnessTestActivePercentDefault, "Brightness in percent for the active output during the test (0-100)")

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
