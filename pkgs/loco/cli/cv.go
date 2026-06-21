package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/loco/app"
	"github.com/spf13/cobra"
)

func NewCVCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "cv",
		Short: "Read & Write CVs on the locomotives using a command station",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewSetCommand(app))
	command.AddCommand(NewGetCommand(app))
	return command
}

func NewSetCommand(app *app.LocoApp) *cobra.Command {
	type SetArgs struct {
		LocoId  uint8
		Cv      uint8
		Value   uint16
		Track   string
		Verify  bool
		Timeout uint16
		Settle  uint16
	}

	cmdArgs := SetArgs{}
	command := &cobra.Command{
		Use:   "set",
		Short: "Send a CV value to the decoder",
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			// mode selection and validation
			track, trackErr := TrackOrDefault(cmdArgs.Track, cmdArgs.LocoId)
			if trackErr != nil {
				return trackErr
			}

			// Join all args as CV string
			cvString, parseErr := parseArgsAsCVs(args)
			if parseErr != nil {
				return parseErr
			}

			return app.SendCVAction(track, cmdArgs.LocoId, cvString, cmdArgs.Verify, time.Second*time.Duration(cmdArgs.Timeout), time.Millisecond*time.Duration(cmdArgs.Settle))
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().Uint16VarP(&cmdArgs.Settle, "settle", "", 300, "Time in miliseconds between writes")
	command.Flags().BoolVarP(&cmdArgs.Verify, "verify", "", false, "Verify the value after writting")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Use locomotive under specific address")
	command.Flags().StringVarP(&cmdArgs.Track, "track", "t", "", "Track type: 'pom' for programming on main, 'prog' for programming track, or empty for automatic selection")

	return command
}

func NewGetCommand(app *app.LocoApp) *cobra.Command {
	type GetArgs struct {
		LocoId  uint8
		Track   string
		Verify  bool
		Timeout uint16
		Retries uint8
	}

	cmdArgs := GetArgs{}
	command := &cobra.Command{
		Use:   "get",
		Short: "Retrieve a CV value from the decoder",
		Args:  cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			// mode selection and validation
			track, trackErr := TrackOrDefault(cmdArgs.Track, cmdArgs.LocoId)
			if trackErr != nil {
				return trackErr
			}

			// Join all args as CV string
			cvString, parseErr := parseArgsAsCVs(args)
			if parseErr != nil {
				return parseErr
			}

			reads, err := app.ReadCVAction(track, cmdArgs.LocoId, cvString, cmdArgs.Verify, time.Second*time.Duration(cmdArgs.Timeout), cmdArgs.Retries)
			if err != nil {
				return err
			}
			return printCVReads(reads)
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "Connection timeout")
	command.Flags().BoolVarP(&cmdArgs.Verify, "verify", "", false, "Verify the value after writting")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Use locomotive under specific address")
	command.Flags().Uint8VarP(&cmdArgs.Retries, "retry", "", 0, "Retry request multiple times if required")
	command.Flags().StringVarP(&cmdArgs.Track, "track", "t", "", "Track type: 'pom' for programming on main, 'prog' for programming track, or empty for automatic selection")

	return command
}

// TrackOrDefault resolves track mode from flags (pom / prog / empty).
func TrackOrDefault(chosenTrack string, locoId uint8) (string, error) {
	track := chosenTrack
	if track != "" && track != "pom" && track != "prog" {
		return "", fmt.Errorf("invalid track type: %s. Must be either 'pom', 'prog' or empty", track)
	}
	if track == "" {
		track = "pom"
		if locoId == 0 {
			track = "prog"
		}
	}
	return track, nil
}

func parseArgsAsCVs(args []string) (string, error) {
	// read data from stdin if "-- -" was specified at the end of the commandline arguments
	stdinString := ""
	if len(args) >= 1 && args[len(args)-1] == "-" {
		// remove "-- -" form the arguments
		args = args[:len(args)-1]

		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read from stdin: %v", err)
		}
		stdinString = strings.Trim(strings.ReplaceAll(string(data), "\n", ", "), ", ")
		args = append(args, "") // hack to pass the args > 0 validation later
	}

	if len(args) == 0 {
		return "", fmt.Errorf("no CV argument provided")
	}

	// parse
	cvString := args[0]
	if len(args) > 1 {
		cvString = ""
		for i, a := range args {
			if strings.Trim(a, " ") == "" {
				continue
			}
			if i > 0 {
				cvString += " "
			}
			cvString += a
		}
	}

	completeString := cvString
	if stdinString != "" {
		completeString = completeString + ", " + stdinString
	}

	return completeString, nil
}
