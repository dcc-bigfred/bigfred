package cli

import (
	"errors"
	"fmt"

	"github.com/keskad/loco/pkgs/app"
	"github.com/spf13/cobra"
)

func NewAppCommand(a *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "app",
		Short: "Application-level commands",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewAppRailroadCommand(a))

	return command
}

func NewAppRailroadCommand(a *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "railroad",
		Short: "Railroad management commands",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewAppRailroadCpCommand(a))

	return command
}

func NewAppRailroadCpCommand(a *app.LocoApp) *cobra.Command {
	type Args struct {
		LocoName string
	}
	cmdArgs := Args{}

	command := &cobra.Command{
		Use:     "cp <src.db> <dst.db>",
		Short:   "Copy a loco entry from one Railroad App database to another",
		Example: "  loco app railroad cp db1.db --src \"SP45-090\" db2.db",
		Args:    cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			if cmdArgs.LocoName == "" {
				return fmt.Errorf("--src is required")
			}
			return a.RailroadCp(app.RailroadCpArgs{
				SrcFile:  args[0],
				DstFile:  args[1],
				LocoName: cmdArgs.LocoName,
			})
		},
	}

	command.Flags().BoolVarP(&a.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().StringVar(&cmdArgs.LocoName, "src", "", "Name (text field) of the loco to copy")

	return command
}
