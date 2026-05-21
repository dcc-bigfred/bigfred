package app

import (
	"fmt"

	"github.com/keskad/loco/pkgs/loco/commandstation"
	"github.com/keskad/loco/pkgs/loco/config"
	"github.com/keskad/loco/pkgs/loco/output"
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
	Station commandstation.Station

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

// InitializeCommandStation opens a connection to the configured command station.
func (app *LocoApp) InitializeCommandStation() error {
	logrus.Debug("Initializing command station")
	if app.Config.Server.Type == "z21" {
		cmd, cmdErr := commandstation.NewZ21Roco(app.Config.Server.Address, app.Config.Server.Port)
		app.Station = cmd
		if cmdErr != nil {
			return fmt.Errorf("cannot initialize app: %s", cmdErr)
		}
	} else if app.Config.Server.Type == "loconet" {
		switch app.Config.Server.Conn {
		case "", "serial":
			cmd, cmdErr := commandstation.NewLocoNetSerial(app.Config.Server.Device, app.Config.Server.Baudrate)
			app.Station = cmd
			if cmdErr != nil {
				return fmt.Errorf("cannot initialize app: %s", cmdErr)
			}
		case "tcp":
			cmd, cmdErr := commandstation.NewLocoNetTCP(app.Config.Server.Address, app.Config.Server.Port)
			app.Station = cmd
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
