package app

import (
	"fmt"
	"time"

	"github.com/keskad/loco/pkgs/loco/decoders"
)

func (app *LocoApp) FactoryResetAction(locoId uint8, preserveAddr bool, timeout time.Duration, settle time.Duration, recovery time.Duration, retries uint8) (FactoryResetResult, error) {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return FactoryResetResult{}, cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)
	id, err := decoders.Identify(cv)
	if err != nil {
		return FactoryResetResult{}, err
	}

	resetValue, err := decoders.FactoryResetCVValue(id.Kind)
	if err != nil {
		return FactoryResetResult{}, err
	}

	result := FactoryResetResult{
		Decoder:       id,
		ResetCV8Value: resetValue,
	}

	if preserveAddr {
		savedAddr, err := app.readAddressInfo(locoId, timeout, retries)
		if err != nil {
			return FactoryResetResult{}, fmt.Errorf("failed to read address before reset: %w", err)
		}
		result.Preserved = &savedAddr
	}

	if err := app.writeProgCV(locoId, 8, resetValue, timeout); err != nil {
		return FactoryResetResult{}, fmt.Errorf("failed to write CV8 (factory reset): %w", err)
	}

	if recovery > 0 {
		time.Sleep(recovery)
	}

	if preserveAddr && result.Preserved != nil {
		if err := app.writeAddress(locoId, result.Preserved.Address, timeout, settle); err != nil {
			return FactoryResetResult{}, fmt.Errorf("failed to restore address after reset: %w", err)
		}
		result.Restored = true
	}

	return result, nil
}
