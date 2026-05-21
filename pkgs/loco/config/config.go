package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Server struct {
	Address string
	Port    uint16
	Type    string
	// Conn selects connection type for a given command station type (e.g. loconet: serial|tcp).
	Conn string
	// Device is a serial device path (e.g. /dev/ttyACM0) when Conn==serial.
	Device string
	// Baudrate is serial speed when Conn==serial (e.g. 57600).
	Baudrate int
}

type Configuration struct {
	Server Server

	// CurrentLoco describes a contextual configuration of current locomotive
	Loco Loco
}

type Loco struct {
	LocoAddr         uint16
	DecoderType      string
	RailboxSoundSlot uint8
}

// LocoAddr represents locomotive address
type LocoAddr uint16

func NewConfig() (*Configuration, error) {
	config := Configuration{}
	config.Loco = Loco{}

	// application configuration
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName(".loco")
	v.AddConfigPath("$HOME/")
	v.AddConfigPath(".")
	_ = v.SafeWriteConfig()

	v.SetDefault("server.address", "192.168.0.111")
	v.SetDefault("server.port", 21105)
	v.SetDefault("server.type", "z21")
	v.SetDefault("server.conn", "serial")
	v.SetDefault("server.device", "/dev/ttyACM0")
	v.SetDefault("server.baudrate", 57600)

	// contextual locomotive configuration (when current working directory is a locomotive directory that contains loco.json file)
	l := viper.New()
	l.SetConfigType("json")
	l.SetConfigName("loco")
	l.AddConfigPath(".")
	l.ReadInConfig()

	// read both configuration files
	if err := v.ReadInConfig(); err != nil {
		return &Configuration{}, fmt.Errorf("cannot parse config: %s", err.Error())
	}
	if err := v.Unmarshal(&config); err != nil {
		return &config, fmt.Errorf("cannot parse config: %s", err.Error())
	}
	if err := l.ReadInConfig(); err != nil {
		// make loco.json fully optional
		if !strings.Contains(err.Error(), "Not Found") {
			return &Configuration{}, fmt.Errorf("cannot parse config: %s", err.Error())
		}
	}
	if err := l.Unmarshal(&config.Loco); err != nil {
		return &config, fmt.Errorf("cannot parse config: %s", err.Error())
	}

	return &config, nil
}
