package app

import (
	"fmt"

	"github.com/keskad/loco/pkgs/output"

	"github.com/keskad/loco/pkgs/commandstation"
	"github.com/keskad/loco/pkgs/config"
	"github.com/sirupsen/logrus"
)

//
// Actions - a controller level
// prints are allowed only via Printer interface
//
// The controller level is intended to provide a layer of performing actions - everything needed to perform a single action e.g. Read list of given CV's
//

type LocoApp struct {
	Config  *config.Configuration
	station commandstation.Station

	// runtime parameters
	Debug bool
	P     output.Printer
}

// Initialize is running after parsing the arguments, so we know how to configure the app
func (app *LocoApp) Initialize() error {
	// logging
	if app.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// configuration
	logrus.Debug("Reading configuration files")
	cfg, cfgErr := config.NewConfig()
	app.Config = cfg
	if cfgErr != nil {
		return fmt.Errorf("cannot initialize app: %s", cfgErr)
	}
	return nil
}

func (app *LocoApp) initializeCommandStation() error {
	// initialize Command Station communication
	logrus.Debug("Initializing command station")
	if app.Config.Server.Type == "z21" {
		cmd, cmdErr := commandstation.NewZ21Roco(app.Config.Server.Address, app.Config.Server.Port)
		app.station = cmd
		if cmdErr != nil {
			return fmt.Errorf("cannot initialize app: %s", cmdErr)
		}
	} else if app.Config.Server.Type == "loconet" {
		switch app.Config.Server.Conn {
		case "", "serial":
			cmd, cmdErr := commandstation.NewLocoNetSerial(app.Config.Server.Device, app.Config.Server.Baudrate)
			app.station = cmd
			if cmdErr != nil {
				return fmt.Errorf("cannot initialize app: %s", cmdErr)
			}
		case "tcp":
			cmd, cmdErr := commandstation.NewLocoNetTCP(app.Config.Server.Address, app.Config.Server.Port)
			app.station = cmd
			if cmdErr != nil {
				return fmt.Errorf("cannot initialize app: %s", cmdErr)
			}
		default:
			return fmt.Errorf("unknown loconet connection type '%s' (expected serial|tcp)", app.Config.Server.Conn)
		}
	} else {
		return fmt.Errorf("unknown command station type '%s'", app.Config.Server.Type)
	}
	return nil
}
