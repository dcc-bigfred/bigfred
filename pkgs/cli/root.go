package cli

import (
	"errors"

	"github.com/keskad/loco/pkgs/app"
	"github.com/spf13/cobra"
)

func NewRootCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "loco",
		Short: "Unofficial Railbox Command Station & Decoder CLI",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewCVCommand(app))
	command.AddCommand(NewFnCommand(app))
	command.AddCommand(NewSpeedCommand(app))
	command.AddCommand(NewDecoderCommand(app))
	command.AddCommand(NewAppCommand(app))

	return command
}
