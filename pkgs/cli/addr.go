package cli

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/keskad/loco/pkgs/app"
	"github.com/spf13/cobra"
)

const (
	shortAddressMin = 1
	shortAddressMax = 127
	longAddressMin  = 0
	longAddressMax  = 10239
)

func NewAddrCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "addr",
		Short: "Set locomotive short or long DCC address",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewAddrSetCommand(app))
	return command
}

func NewAddrSetCommand(app *app.LocoApp) *cobra.Command {
	type SetArgs struct {
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

			return app.SendCVAction(
				"prog",
				0,
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

	return command
}

func addressToCVString(addr uint16) (string, error) {
	if addr < longAddressMin || addr > longAddressMax {
		return "", fmt.Errorf("address %d out of range (%d-%d)", addr, longAddressMin, longAddressMax)
	}

	if addr >= shortAddressMin && addr <= shortAddressMax {
		return fmt.Sprintf("cv1=%d, cv17=0, cv18=0, cv29=0", addr), nil
	}

	cv17 := 192 + (addr / 256)
	cv18 := addr % 256
	return fmt.Sprintf("cv17=%d, cv18=%d, cv29=32", cv17, cv18), nil
}
