package app

import "github.com/keskad/loco/pkgs/loco/decoders"

// CVRead holds the result of reading one configuration variable.
type CVRead struct {
	Number uint16
	Value  int
	Err    error
}

// OutputBrightnessLevel is brightness of one decoder output in percent.
type OutputBrightnessLevel struct {
	Output     uint8
	Brightness uint8
}

// FactoryResetResult describes a completed factory reset.
type FactoryResetResult struct {
	Decoder       decoders.Identification
	ResetCV8Value int
	Preserved     *AddressInfo
	Restored      bool
}

// FunctionMappingResult describes programmed function-to-output mapping.
type FunctionMappingResult struct {
	Function  uint8
	Outputs   []string
	Direction string
	Writes    []decoders.MappingWrite
}

// RailroadCpResult describes the outcome of copying a loco between databases.
type RailroadCpResult struct {
	LocoName string
	DstFile  string
	Updated  bool
	ID       int64
}
