package main

import (
	"os"

	"github.com/dcc-bigfred/bigfred/pkgs/loco/app"
	"github.com/dcc-bigfred/bigfred/pkgs/loco/cli"
	"github.com/dcc-bigfred/bigfred/pkgs/loco/output"
)

func main() {
	app := app.LocoApp{P: output.ConsolePrinter{}}
	cmd := cli.NewRootCommand(&app)
	args := os.Args
	if args != nil {
		args = args[1:]
		cmd.SetArgs(args)
	}
	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
