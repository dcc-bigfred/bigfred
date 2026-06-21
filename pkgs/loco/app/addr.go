package app

import (
	"fmt"
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
	"github.com/keskad/loco/pkgs/loco/syntax"
)

const cv29LongAddressBit = 32

const (
	shortAddressMin = 1
	shortAddressMax = 127
	longAddressMin  = 0
	longAddressMax  = 10239
)

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

// AddressToCVString builds CV write assignments for programming a decoder address.
func AddressToCVString(addr uint16) (string, error) {
	if addr < longAddressMin || addr > longAddressMax {
		return "", fmt.Errorf("address %d out of range (%d-%d)", addr, longAddressMin, longAddressMax)
	}

	if addr >= shortAddressMin && addr <= shortAddressMax {
		return fmt.Sprintf("cv1=%d, cv17=0, cv18=0, cv29=0", addr), nil
	}

	cv17 := 192 + (addr / 256)
	cv18 := addr % 256
	return fmt.Sprintf("cv17=%d, cv18=%d, cv29=32", cv17, cv18), nil
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

func (app *LocoApp) writeProgCV(locoId uint8, num uint16, value int, timeout time.Duration) error {
	return app.Station.WriteCV(
		progModeForLoco(locoId),
		commandstation.LocoCV{
			LocoId: commandstation.LocoAddr(locoId),
			Cv:     commandstation.CV{Num: commandstation.CVNum(num), Value: value},
		},
		commandstation.Timeout(timeout),
	)
}

func (app *LocoApp) readAddressInfo(locoId uint8, timeout time.Duration, retries uint8) (AddressInfo, error) {
	cv1, err := app.readProgCV(locoId, 1, timeout, retries)
	if err != nil {
		return AddressInfo{}, fmt.Errorf("failed to read CV1: %w", err)
	}
	cv17, err := app.readProgCV(locoId, 17, timeout, retries)
	if err != nil {
		return AddressInfo{}, fmt.Errorf("failed to read CV17: %w", err)
	}
	cv18, err := app.readProgCV(locoId, 18, timeout, retries)
	if err != nil {
		return AddressInfo{}, fmt.Errorf("failed to read CV18: %w", err)
	}
	cv29, err := app.readProgCV(locoId, 29, timeout, retries)
	if err != nil {
		return AddressInfo{}, fmt.Errorf("failed to read CV29: %w", err)
	}
	return addressFromCVs(cv1, cv17, cv18, cv29)
}

func (app *LocoApp) writeAddress(locoId uint8, addr uint16, timeout time.Duration, settle time.Duration) error {
	cvString, err := AddressToCVString(addr)
	if err != nil {
		return err
	}

	entries, err := syntax.ParseCVString(cvString, ",")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := app.writeProgCV(locoId, entry.Number, int(entry.Value), timeout); err != nil {
			return fmt.Errorf("failed to write CV%d: %w", entry.Number, err)
		}
		time.Sleep(settle)
	}
	return nil
}

func (app *LocoApp) GetAddrAction(locoId uint8, timeout time.Duration, retries uint8) (AddressInfo, error) {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return AddressInfo{}, cmdErr
	}
	defer app.Station.CleanUp()

	return app.readAddressInfo(locoId, timeout, retries)
}
