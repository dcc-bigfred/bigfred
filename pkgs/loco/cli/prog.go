package cli

import (
	"errors"

	"github.com/dcc-bigfred/bigfred/pkgs/loco/app"
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
	command.AddCommand(NewAddrCommand(app))
	return command
}
