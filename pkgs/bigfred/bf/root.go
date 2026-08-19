package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/keskad/loco/pkgs/bigfred/server/ctl"
	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/version"
)

type options struct {
	socket string
	output string
}

func newRootCommand() *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:           "bf",
		Short:         "Talk to a running loco-server over its Unix control socket.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			switch opts.output {
			case "human", "json":
				return nil
			default:
				return fmt.Errorf("invalid output format %q: must be human or json", opts.output)
			}
		},
	}
	cmd.PersistentFlags().StringVar(&opts.socket, "socket", datadir.Path("run", "bigfred.sock"),
		"loco-server control socket (default $BIGFRED_DATA_DIR/run/bigfred.sock)")
	cmd.PersistentFlags().StringVarP(&opts.output, "output", "o", "human",
		"output format: human|json")

	layouts := &cobra.Command{Use: "layouts", Short: "Layout catalogue"}
	layouts.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List layouts",
		RunE: func(*cobra.Command, []string) error {
			return runLayoutsList(opts)
		},
	})
	dccBus := &cobra.Command{Use: "dcc-bus", Short: "dcc-bus programs"}
	dccBus.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List dcc-bus programs",
		RunE: func(*cobra.Command, []string) error {
			return runDccBusList(opts)
		},
	})
	cmd.AddCommand(layouts, dccBus, &cobra.Command{
		Use:   "version",
		Short: "Show the running loco-server version",
		RunE: func(*cobra.Command, []string) error {
			return runVersion(opts)
		},
	})
	return cmd
}

func runLayoutsList(opts options) error {
	raw, err := ctl.Call(opts.socket, "layouts_list")
	if err != nil {
		return err
	}
	if opts.output == "json" {
		return printJSON(raw)
	}
	var rows []protocol.LayoutResponse
	if err := json.Unmarshal(raw, &rows); err != nil {
		return err
	}
	printLayoutsHuman(os.Stdout, rows)
	return nil
}

func runDccBusList(opts options) error {
	raw, err := ctl.Call(opts.socket, "dcc_bus_list")
	if err != nil {
		return err
	}
	if opts.output == "json" {
		return printJSON(raw)
	}
	var body ctl.DccBusListResponse
	if err := json.Unmarshal(raw, &body); err != nil {
		return err
	}
	printDccBusHuman(os.Stdout, body.Programs)
	return nil
}

func runVersion(opts options) error {
	raw, err := ctl.Call(opts.socket, "version")
	if err != nil {
		return err
	}
	if opts.output == "json" {
		return printJSON(raw)
	}
	var info version.Info
	if err := json.Unmarshal(raw, &info); err != nil {
		return err
	}
	printVersionHuman(os.Stdout, info)
	return nil
}

func printJSON(raw []byte) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
