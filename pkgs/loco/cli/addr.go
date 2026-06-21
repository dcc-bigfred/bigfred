package cli

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/keskad/loco/pkgs/loco/app"
	"github.com/spf13/cobra"
)

func NewAddrCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "addr",
		Short: "Get or set locomotive short or long DCC address",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewAddrGetCommand(app))
	command.AddCommand(NewAddrSetCommand(app))
	return command
}

func NewAddrGetCommand(app *app.LocoApp) *cobra.Command {
	type GetArgs struct {
		LocoId  uint8
		Timeout uint16
		Retries uint8
	}

	cmdArgs := GetArgs{}
	command := &cobra.Command{
		Use:   "get",
		Short: "Read decoder address from CV1, CV17, CV18 and CV29",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			info, err := app.GetAddrAction(
				cmdArgs.LocoId,
				time.Second*time.Duration(cmdArgs.Timeout),
				cmdArgs.Retries,
			)
			if err != nil {
				return err
			}
			printAddressInfo(info)
			return nil
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint8VarP(&cmdArgs.Retries, "retry", "", 0, "Retry request multiple times if required")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)

	return command
}

func NewAddrSetCommand(app *app.LocoApp) *cobra.Command {
	type SetArgs struct {
		LocoId  uint8
		Verify  bool
		Timeout uint16
		Settle  uint16
	}

	cmdArgs := SetArgs{}
	command := &cobra.Command{
		Use:   "set <address>",
		Short: "Program decoder short or long address",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			addr64, parseErr := strconv.ParseUint(args[0], 10, 16)
			if parseErr != nil {
				return fmt.Errorf("invalid address %q: %w", args[0], parseErr)
			}

			cvString, buildErr := addressToCVString(uint16(addr64))
			if buildErr != nil {
				return buildErr
			}

			track, trackErr := TrackOrDefault("", cmdArgs.LocoId)
			if trackErr != nil {
				return trackErr
			}

			return app.SendCVAction(
				track,
				cmdArgs.LocoId,
				cvString,
				cmdArgs.Verify,
				time.Second*time.Duration(cmdArgs.Timeout),
				time.Millisecond*time.Duration(cmdArgs.Settle),
			)
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint16VarP(&cmdArgs.Settle, "settle", "", 300, "Time in miliseconds between writes")
	command.Flags().BoolVarP(&cmdArgs.Verify, "verify", "", false, "Verify the value after writting")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, progLocoFlagUsage)

	return command
}

func addressToCVString(addr uint16) (string, error) {
	return app.AddressToCVString(addr)
}
