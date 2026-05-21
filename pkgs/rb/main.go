package main

import (
	"os"

	"github.com/keskad/loco/pkgs/loco/app"
	"github.com/keskad/loco/pkgs/loco/output"
	rbcli "github.com/keskad/loco/pkgs/rb/cli"
)

func main() {
	loc := app.LocoApp{P: output.ConsolePrinter{}}
	cmd := rbcli.NewRootCommand(&loc)
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
