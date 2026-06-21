package rbapp

import (
	"errors"
	"fmt"
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// LNCVArgs configures rb lncv set/get.
type LNCVArgs struct {
	Device     string
	Baudrate   int
	Article    int
	ModuleAddr int
	CV         int
	Timeout    time.Duration
	// SelfConfig writes the adapter's own configuration (e.g. Uhlenbrock 63120
	// CV2 baud / CV4 mode), which is applied without an acknowledge.
	SelfConfig bool
}

// LNCVWriteResult describes a completed LNCV write.
type LNCVWriteResult struct {
	CV           int
	Value        int
	Article      int
	ModuleAddr   int
	SelfConfig   bool
	AppliedNoAck bool
}

func openLNCVLocoNet(args LNCVArgs) (*commandstation.LocoNet, error) {
	if args.Device == "" {
		return nil, fmt.Errorf("serial device is required (e.g. /dev/ttyACM0)")
	}
	if args.Baudrate <= 0 {
		return nil, fmt.Errorf("invalid baud rate %d", args.Baudrate)
	}

	ln, err := commandstation.NewLocoNetSerial(args.Device, args.Baudrate)
	if err != nil {
		return nil, err
	}
	if args.Timeout > 0 {
		ln.SetTimeout(args.Timeout)
	}
	return ln, nil
}

// LNCVSet opens a LocoNet serial link and writes one LNCV on the target module.
func LNCVSet(args LNCVArgs, value int) (LNCVWriteResult, error) {
	ln, err := openLNCVLocoNet(args)
	if err != nil {
		return LNCVWriteResult{}, err
	}
	defer ln.CleanUp()

	article := commandstation.NormalizeLncvArticle(args.Article)
	result := LNCVWriteResult{
		CV:         args.CV,
		Value:      value,
		Article:    article,
		ModuleAddr: args.ModuleAddr,
		SelfConfig: args.SelfConfig,
	}

	if args.SelfConfig {
		err := ln.SetLNCVSelfConfig(args.Article, args.ModuleAddr, args.CV, value)
		switch {
		case errors.Is(err, commandstation.ErrLncvAppliedNoAck):
			result.AppliedNoAck = true
			return result, nil
		case err != nil:
			return LNCVWriteResult{}, err
		}
		return result, nil
	}

	if err := ln.SetLNCV(args.Article, args.ModuleAddr, args.CV, value); err != nil {
		return LNCVWriteResult{}, err
	}

	return result, nil
}

// LNCVGet reads one LNCV from a LocoNet module.
func LNCVGet(args LNCVArgs) (int, error) {
	ln, err := openLNCVLocoNet(args)
	if err != nil {
		return 0, err
	}
	defer ln.CleanUp()

	return ln.ReadLNCV(args.Article, args.ModuleAddr, args.CV)
}
