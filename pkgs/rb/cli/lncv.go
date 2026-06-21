package cli

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	locoapp "github.com/keskad/loco/pkgs/loco/app"
	rbapp "github.com/keskad/loco/pkgs/rb/app"
	"github.com/spf13/cobra"
)

type lncvCmdArgs struct {
	Article    int
	ModuleAddr int
	Device     string
	Baudrate   int
	Timeout    uint16
	SelfConfig bool
}

func defaultLncvCmdArgs() lncvCmdArgs {
	return lncvCmdArgs{
		Article:    6312,
		ModuleAddr: 1,
		Baudrate:   115200,
	}
}

func (a lncvCmdArgs) connArgs(cv int) rbapp.LNCVArgs {
	return rbapp.LNCVArgs{
		Device:     a.Device,
		Baudrate:   a.Baudrate,
		Article:    a.Article,
		ModuleAddr: a.ModuleAddr,
		CV:         cv,
		Timeout:    time.Second * time.Duration(a.Timeout),
		SelfConfig: a.SelfConfig,
	}
}

func addLNCVFlags(cmd *cobra.Command, loc *locoapp.LocoApp, args *lncvCmdArgs) {
	cmd.Flags().BoolVarP(&loc.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	cmd.Flags().IntVarP(&args.Article, "article", "a", args.Article, "LNCV article number (6312 for Uhlenbrock 63120; 63120 is accepted)")
	cmd.Flags().IntVar(&args.ModuleAddr, "addr", args.ModuleAddr, "Module address on LocoNet (LNCV 0, default 1)")
	cmd.Flags().StringVar(&args.Device, "device", "", "Serial device (default: server.device from ~/.loco.yaml)")
	cmd.Flags().IntVar(&args.Baudrate, "baud", args.Baudrate, "Serial baud rate (factory default for 63120: 115200)")
	cmd.Flags().Uint16Var(&args.Timeout, "timeout", 4, "LocoNet response timeout in seconds")
}

func resolveLncvDevice(loc *locoapp.LocoApp, device string) string {
	if device != "" {
		return device
	}
	return loc.Config.Server.Device
}

func NewLNCVCommand(loc *locoapp.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "lncv",
		Short: "Program Uhlenbrock LNCV modules over LocoNet (e.g. Uhlenbrock 63120)",
		Long: `Read and write LocoNet Configuration Variables (LNCV) on Uhlenbrock-class modules.

Requires a powered LocoNet bus, the module connected to LocoNet, and a USB
LocoNet adapter (e.g. Uhlenbrock 63120) on the host. Use factory baud 115200
until LNCV 2 is set to 57600.

Example — set Uhlenbrock 63120 to 57600 baud + Direktmodus:
  rb lncv set --device /dev/ttyACM0 --baud 115200 4 1
  rb lncv set --device /dev/ttyACM0 --baud 115200 2 3

Example — verify current settings:
  rb lncv get --device /dev/ttyACM0 --baud 57600 2
  rb lncv get --device /dev/ttyACM0 --baud 57600 4`,
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewLNCVSetCommand(loc))
	command.AddCommand(NewLNCVGetCommand(loc))
	return command
}

func NewLNCVSetCommand(loc *locoapp.LocoApp) *cobra.Command {
	cmdArgs := defaultLncvCmdArgs()

	command := &cobra.Command{
		Use:   "set <cv> <value>",
		Short: "Write an LNCV on a LocoNet module",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			if err := loc.Initialize(); err != nil {
				return err
			}

			cv, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid CV number %q: %w", args[0], err)
			}
			val, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid value %q: %w", args[1], err)
			}

			conn := cmdArgs.connArgs(cv)
			conn.Device = resolveLncvDevice(loc, cmdArgs.Device)
			result, err := rbapp.LNCVSet(conn, val)
			if err != nil {
				return err
			}
			printLNCVWriteResult(result)
			return nil
		},
	}

	addLNCVFlags(command, loc, &cmdArgs)
	command.Flags().BoolVar(&cmdArgs.SelfConfig, "self-config", false,
		"Write the adapter's own configuration (e.g. 63120 CV2 baud / CV4 mode); applied without acknowledge")
	return command
}

func NewLNCVGetCommand(loc *locoapp.LocoApp) *cobra.Command {
	cmdArgs := defaultLncvCmdArgs()

	command := &cobra.Command{
		Use:   "get <cv>",
		Short: "Read an LNCV from a LocoNet module",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := loc.Initialize(); err != nil {
				return err
			}

			cv, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid CV number %q: %w", args[0], err)
			}

			conn := cmdArgs.connArgs(cv)
			conn.Device = resolveLncvDevice(loc, cmdArgs.Device)
			val, err := rbapp.LNCVGet(conn)
			if err != nil {
				return err
			}
			fmt.Printf("%d\n", val)
			return nil
		},
	}

	addLNCVFlags(command, loc, &cmdArgs)
	return command
}
