package cli

import (
	"fmt"

	"github.com/keskad/loco/pkgs/loco/app"
	"github.com/keskad/loco/pkgs/loco/decoders"
)

func printAddressInfo(info app.AddressInfo) {
	fmt.Printf("cv1=%d\n", info.CV1)
	fmt.Printf("cv17=%d\n", info.CV17)
	fmt.Printf("cv18=%d\n", info.CV18)
	fmt.Printf("cv29=%d\n", info.CV29)
	fmt.Printf("address=%d\n", info.Address)
	fmt.Printf("type=%s\n", info.Type)
}

func printDecoderIdentification(id decoders.Identification) {
	if id.SoftwareVersion >= 0 {
		fmt.Printf("cv7=%d\n", id.SoftwareVersion)
	}
	fmt.Printf("cv8=%d\n", id.ManufacturerID)
	fmt.Printf("decoder=%s\n", id.Name)
}

func printFactoryResetResult(result app.FactoryResetResult) {
	if result.Preserved != nil {
		fmt.Printf("preserving address=%d (%s)\n", result.Preserved.Address, result.Preserved.Type)
	}
	fmt.Printf("decoder=%s\n", result.Decoder.Name)
	fmt.Printf("factory reset: cv8=%d\n", result.ResetCV8Value)
	if result.Restored && result.Preserved != nil {
		fmt.Printf("restoring address=%d\n", result.Preserved.Address)
	}
	fmt.Printf("factory reset complete\n")
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

func printBrightnessLevels(levels []app.OutputBrightnessLevel) {
	for _, level := range levels {
		fmt.Printf("output=%d brightness=%d\n", level.Output, level.Brightness)
	}
}

func printBrightnessSnapshot(snapshot []decoders.OutputBrightness) {
	fmt.Printf("Saved brightness values:\n")
	for _, state := range snapshot {
		fmt.Printf("output=%d cv%d=%d\n", state.Output, state.CV, state.Value)
	}
}
