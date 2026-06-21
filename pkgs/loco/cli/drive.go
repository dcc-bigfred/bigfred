package cli

import (
	"errors"

	"github.com/keskad/loco/pkgs/loco/app"
	"github.com/spf13/cobra"
)

func NewDriveCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "drive",
		Short: "Control locomotives on the main track",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewSpeedCommand(app))
	command.AddCommand(NewFnCommand(app))
	return command
}
