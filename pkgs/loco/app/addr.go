package app

import (
	"fmt"
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const cv29LongAddressBit = 32

// AddressInfo holds decoded locomotive address data read from CVs.
type AddressInfo struct {
	CV1     int
	CV17    int
	CV18    int
	CV29    int
	Address uint16
	Type    string // "short" or "long"
}

func addressFromCVs(cv1, cv17, cv18, cv29 int) (AddressInfo, error) {
	info := AddressInfo{CV1: cv1, CV17: cv17, CV18: cv18, CV29: cv29}

	if cv29&cv29LongAddressBit != 0 {
		if cv17 < 192 {
			return AddressInfo{}, fmt.Errorf("invalid long address: CV17=%d (expected >= 192)", cv17)
		}
		info.Address = uint16((cv17-192)*256 + cv18)
		info.Type = "long"
		return info, nil
	}

	if cv1 < 1 || cv1 > 127 {
		return AddressInfo{}, fmt.Errorf("invalid short address: CV1=%d (expected 1-127)", cv1)
	}
	info.Address = uint16(cv1)
	info.Type = "short"
	return info, nil
}

func (app *LocoApp) readProgCV(locoId uint8, num uint16, timeout time.Duration, retries uint8) (int, error) {
	return app.Station.ReadCV(
		progModeForLoco(locoId),
		commandstation.LocoCV{
			LocoId: commandstation.LocoAddr(locoId),
			Cv:     commandstation.CV{Num: commandstation.CVNum(num)},
		},
		commandstation.Timeout(timeout),
		commandstation.Retries(retries),
	)
}

func (app *LocoApp) GetAddrAction(locoId uint8, timeout time.Duration, retries uint8) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()

	cv1, err := app.readProgCV(locoId, 1, timeout, retries)
	if err != nil {
		return fmt.Errorf("failed to read CV1: %w", err)
	}
	cv17, err := app.readProgCV(locoId, 17, timeout, retries)
	if err != nil {
		return fmt.Errorf("failed to read CV17: %w", err)
	}
	cv18, err := app.readProgCV(locoId, 18, timeout, retries)
	if err != nil {
		return fmt.Errorf("failed to read CV18: %w", err)
	}
	cv29, err := app.readProgCV(locoId, 29, timeout, retries)
	if err != nil {
		return fmt.Errorf("failed to read CV29: %w", err)
	}

	info, err := addressFromCVs(cv1, cv17, cv18, cv29)
	if err != nil {
		return err
	}

	app.P.Printf("cv1=%d\n", info.CV1)
	app.P.Printf("cv17=%d\n", info.CV17)
	app.P.Printf("cv18=%d\n", info.CV18)
	app.P.Printf("cv29=%d\n", info.CV29)
	app.P.Printf("address=%d\n", info.Address)
	app.P.Printf("type=%s\n", info.Type)
	return nil
}
