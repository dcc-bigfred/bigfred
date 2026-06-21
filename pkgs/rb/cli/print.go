package cli

import (
	"fmt"

	locoapp "github.com/keskad/loco/pkgs/loco/app"
	rbapp "github.com/keskad/loco/pkgs/rb/app"
)

func printRailroadCpResult(result locoapp.RailroadCpResult) {
	if result.Updated {
		fmt.Printf("Updated loco %q (id=%d) in %s\n", result.LocoName, result.ID, result.DstFile)
		return
	}
	fmt.Printf("Copied loco %q to %s\n", result.LocoName, result.DstFile)
}

func printLNCVWriteResult(result rbapp.LNCVWriteResult) {
	if result.SelfConfig && result.AppliedNoAck {
		fmt.Printf("LNCV %d = %d sent to adapter (article %d). The adapter applies "+
			"self-configuration without an acknowledge; reconnect with the new settings to verify.\n",
			result.CV, result.Value, result.Article)
		return
	}
	if result.SelfConfig {
		fmt.Printf("LNCV %d = %d written and acknowledged (article %d)\n", result.CV, result.Value, result.Article)
		return
	}
	fmt.Printf("LNCV %d = %d written (article %d, module %d)\n",
		result.CV, result.Value, result.Article, result.ModuleAddr)
}
