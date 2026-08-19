package main

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
	"github.com/keskad/loco/pkgs/bigfred/server/version"
)

func printLayoutsHuman(w io.Writer, rows []protocol.LayoutResponse) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tSYSTEM\tLOCKED")
	for _, r := range rows {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", r.ID, r.Name, yesNo(r.IsSystem), yesNo(r.Locked))
	}
	_ = tw.Flush()
}

func printDccBusHuman(w io.Writer, programs []service.DccBusProgramStatus) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tLAYOUT\tSTATUS\tWIT\tZ21")
	for _, p := range programs {
		layout := p.LayoutName
		if layout == "" {
			layout = strconv.FormatUint(uint64(p.LayoutID), 10)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			p.Name, layout, p.Status,
			portOrDash(p.WithrottleEnabled, p.WithrottlePort),
			portOrDash(p.Z21Enabled, p.Z21Port),
		)
	}
	_ = tw.Flush()
}

func printVersionHuman(w io.Writer, info version.Info) {
	fmt.Fprintf(w, "Product:      %s\n", info.Product)
	fmt.Fprintf(w, "Version:      %s\n", dash(info.Version))
	fmt.Fprintf(w, "Tag commit:   %s\n", dash(info.TagCommit))
	fmt.Fprintf(w, "Build commit: %s\n", dash(info.BuildCommit))
	fmt.Fprintf(w, "Build time:   %s\n", dash(info.BuildTime))
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func portOrDash(enabled bool, port uint16) string {
	if !enabled || port == 0 {
		return "-"
	}
	return strconv.FormatUint(uint64(port), 10)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
