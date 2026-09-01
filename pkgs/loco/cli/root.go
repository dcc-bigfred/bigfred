package cli

import (
	"errors"

	"github.com/dcc-bigfred/bigfred/pkgs/loco/app"
	"github.com/spf13/cobra"
)

func NewRootCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "loco",
		Short: "DCC command-station CLI (drive, CV programming, LNCV)",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewDriveCommand(app))
	command.AddCommand(NewProgCommand(app))
	command.AddCommand(NewLNCVCommand(app))

	return command
}
