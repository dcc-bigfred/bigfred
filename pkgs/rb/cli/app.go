package cli

import (
	"errors"
	"fmt"

	locoapp "github.com/keskad/loco/pkgs/loco/app"
	"github.com/spf13/cobra"
)

func NewAppCommand(a *locoapp.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "app",
		Short: "Application-level commands",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewAppCpCommand(a))

	return command
}

func NewAppCpCommand(a *locoapp.LocoApp) *cobra.Command {
	type Args struct {
		LocoName string
	}
	cmdArgs := Args{}

	command := &cobra.Command{
		Use:     "cp <src.db> <dst.db>",
		Short:   "Copy a loco entry from one Railroad App database to another",
		Example: "  rb app cp db1.db --src \"SP45-090\" db2.db",
		Args:    cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			if cmdArgs.LocoName == "" {
				return fmt.Errorf("--src is required")
			}
			result, err := a.RailroadCp(locoapp.RailroadCpArgs{
				SrcFile:  args[0],
				DstFile:  args[1],
				LocoName: cmdArgs.LocoName,
			})
			if err != nil {
				return err
			}
			printRailroadCpResult(result)
			return nil
		},
	}

	command.Flags().BoolVarP(&a.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().StringVar(&cmdArgs.LocoName, "src", "", "Name (text field) of the loco to copy")

	return command
}
