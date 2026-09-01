package cli

import (
	"fmt"

	"github.com/dcc-bigfred/bigfred/pkgs/loco/app"
)

func printAddressInfo(info app.AddressInfo) {
	fmt.Printf("cv1=%d\n", info.CV1)
	fmt.Printf("cv17=%d\n", info.CV17)
	fmt.Printf("cv18=%d\n", info.CV18)
	fmt.Printf("cv29=%d\n", info.CV29)
	fmt.Printf("address=%d\n", info.Address)
	fmt.Printf("type=%s\n", info.Type)
}

func printCVReads(reads []app.CVRead) error {
	if len(reads) == 0 {
		return nil
	}
	if len(reads) == 1 {
		if reads[0].Err != nil {
			return reads[0].Err
		}
		fmt.Printf("%d\n", reads[0].Value)
		return nil
	}

	var lastErr error
	for _, read := range reads {
		if read.Err != nil {
			fmt.Printf("cv%d=ERROR\n", read.Number)
			lastErr = read.Err
			continue
		}
		fmt.Printf("cv%d=%d\n", read.Number, read.Value)
	}
	return lastErr
}

func printCVBitWriteResults(results []app.CVBitWriteResult) {
	for _, result := range results {
		fmt.Printf("cv%d=%d (was %d)\n", result.Number, result.After, result.Before)
	}
}

func printActiveFunctions(functions []int) {
	if len(functions) == 0 {
		fmt.Printf("No active functions\n")
		return
	}
	for _, fnNum := range functions {
		fmt.Printf("F%d = On\n", fnNum)
	}
}

func printLNCVWriteResult(result app.LNCVWriteResult) {
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
