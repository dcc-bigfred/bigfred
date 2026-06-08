package cli

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/keskad/loco/pkgs/loco/decoders"
	locoapp "github.com/keskad/loco/pkgs/loco/app"
	lococli "github.com/keskad/loco/pkgs/loco/cli"
	rbapp "github.com/keskad/loco/pkgs/rb/app"
	"github.com/spf13/cobra"
)

func NewRootCommand(loc *locoapp.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "rb",
		Short: "CLI for Railbox RB23xx decoders",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewSoundCommand(loc))
	command.AddCommand(NewWifiCommand(loc))
	command.AddCommand(NewLNCVCommand(loc))
	command.AddCommand(NewAppCommand(loc))

	return command
}

func NewSoundCommand(loc *locoapp.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "sound",
		Short: "Sound management for Railbox RB23xx decoders",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewSoundClearCommand(loc))
	command.AddCommand(NewSoundSyncCommand(loc))

	return command
}

func NewSoundClearCommand(loc *locoapp.LocoApp) *cobra.Command {
	type Args struct {
		Timeout uint16
	}
	cmdArgs := Args{}

	command := &cobra.Command{
		Use:   "clear <slot>",
		Short: "Clear sound files from a slot on the Railbox RB23xx decoder",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			slot64, err := strconv.ParseUint(args[0], 10, 8)
			if err != nil {
				return fmt.Errorf("invalid slot number %q: %w", args[0], err)
			}

			return rbapp.ClearSoundSlot(uint8(slot64), decoders.WithTimeout(cmdArgs.Timeout))
		},
	}

	command.Flags().BoolVarP(&loc.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "HTTP connection timeout in seconds")

	return command
}

func NewSoundSyncCommand(loc *locoapp.LocoApp) *cobra.Command {
	type Args struct {
		Timeout     uint16
		DryRun      bool
		WithoutLast bool
		Watch       bool
	}
	cmdArgs := Args{}

	command := &cobra.Command{
		Use:   "sync <slot> <local-dir>",
		Short: "Synchronise a local directory with a sound slot on the Railbox RB23xx decoder",
		Long: `Compares the contents of a local directory with the given sound slot on the decoder.
Files present locally but missing on the decoder are uploaded.
Files present on the decoder but missing locally are deleted from the decoder.
Files present on both sides but differing in size are re-uploaded.
By default the 5 most recently modified local files (modified within the last 24 h) are always re-uploaded.
Use --without-last to disable this behaviour.
Use --watch to keep watching the directory and re-sync automatically on every change.`,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			slot64, err := strconv.ParseUint(args[0], 10, 8)
			if err != nil {
				return fmt.Errorf("invalid slot number %q: %w", args[0], err)
			}

			opts := []decoders.Option{decoders.WithTimeout(cmdArgs.Timeout)}

			if cmdArgs.Watch {
				return rbapp.WatchSoundSlot(loc, uint8(slot64), args[1], cmdArgs.DryRun, cmdArgs.WithoutLast, opts...)
			}
			return rbapp.SyncSoundSlot(loc, uint8(slot64), args[1], cmdArgs.DryRun, cmdArgs.WithoutLast, opts...)
		},
	}

	command.Flags().BoolVarP(&loc.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "HTTP connection timeout in seconds")
	command.Flags().BoolVar(&cmdArgs.DryRun, "dry-run", false, "Preview changes without uploading or deleting any files")
	command.Flags().BoolVarP(&cmdArgs.WithoutLast, "without-last", "l", false, "Disable automatic re-upload of the 5 most recently modified files (last 24 h)")
	command.Flags().BoolVarP(&cmdArgs.Watch, "watch", "w", false, "Watch the local directory and re-sync automatically on every file change")

	return command
}

func NewWifiCommand(loc *locoapp.LocoApp) *cobra.Command {
	type Args struct {
		LocoId  uint8
		Track   string
		Timeout uint16
	}
	cmdArgs := Args{}

	command := &cobra.Command{
		Use:   "wifi <on|off>",
		Short: "Turn the WiFi router on or off on a Railbox RB23xx decoder",
		Long: `Reads CV200 to determine which function number controls the built-in WiFi router,
then enables or disables that function on the decoder.`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			switch args[0] {
			case "on", "off":
			default:
				return fmt.Errorf("invalid argument %q: must be 'on' or 'off'", args[0])
			}

			if err := loc.Initialize(); err != nil {
				return err
			}

			track, trackErr := lococli.TrackOrDefault(cmdArgs.Track, cmdArgs.LocoId)
			if trackErr != nil {
				return trackErr
			}

			enable := args[0] == "on"
			return rbapp.RBWifiAction(loc, track, cmdArgs.LocoId, enable, time.Second*time.Duration(cmdArgs.Timeout))
		},
	}

	command.Flags().BoolVarP(&loc.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout in seconds")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Use locomotive under specific address")
	command.Flags().StringVarP(&cmdArgs.Track, "track", "t", "", "Track type: 'pom' for programming on main, 'prog' for programming track, or empty for automatic selection")

	return command
}
