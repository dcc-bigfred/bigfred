package rbapp

import (
	"errors"
	"fmt"
	"time"

	"github.com/keskad/loco/pkgs/loco/app"
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
func LNCVSet(loc *app.LocoApp, args LNCVArgs, value int) error {
	ln, err := openLNCVLocoNet(args)
	if err != nil {
		return err
	}
	defer ln.CleanUp()

	article := commandstation.NormalizeLncvArticle(args.Article)

	if args.SelfConfig {
		err := ln.SetLNCVSelfConfig(args.Article, args.ModuleAddr, args.CV, value)
		switch {
		case errors.Is(err, commandstation.ErrLncvAppliedNoAck):
			_, _ = loc.P.Printf("LNCV %d = %d sent to adapter (article %d). The adapter applies "+
				"self-configuration without an acknowledge; reconnect with the new settings to verify.\n",
				args.CV, value, article)
			return nil
		case err != nil:
			return err
		}
		_, _ = loc.P.Printf("LNCV %d = %d written and acknowledged (article %d)\n", args.CV, value, article)
		return nil
	}

	if err := ln.SetLNCV(args.Article, args.ModuleAddr, args.CV, value); err != nil {
		return err
	}

	_, _ = loc.P.Printf("LNCV %d = %d written (article %d, module %d)\n",
		args.CV, value, article, args.ModuleAddr)
	return nil
}

// LNCVGet reads one LNCV from a LocoNet module.
func LNCVGet(loc *app.LocoApp, args LNCVArgs) (int, error) {
	ln, err := openLNCVLocoNet(args)
	if err != nil {
		return 0, err
	}
	defer ln.CleanUp()

	val, err := ln.ReadLNCV(args.Article, args.ModuleAddr, args.CV)
	if err != nil {
		return 0, err
	}

	_, _ = loc.P.Printf("%d\n", val)
	return val, nil
}
