package decoders

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// OutputBrightness holds the saved brightness state of a single output.
type OutputBrightness struct {
	Output  uint8
	CV      uint16
	Value   int
	Percent uint8
}

// BrightnessChangeable is implemented by decoders that support per-output brightness control.
type BrightnessChangeable interface {
	SetBrightness(output uint8, percent uint8) error
	GetBrightness(output uint8) (uint8, error)
	SetBrightnessRaw(output uint8, value int) error
	SnapshotBrightness() ([]OutputBrightness, error)
	Outputs() []uint8
}

// BrightnessIdentifyHooks drives the interactive brightness identification test.
// CLI sets OnOutput to announce the active output and WaitNext to pause before the next one.
type BrightnessIdentifyHooks struct {
	OnOutput func(state OutputBrightness, index, total int) error
	WaitNext func(state OutputBrightness, index, total int) error
}

// BrightnessTestActivePercentDefault is the brightness used for each output during
// loco prog brightness test when --brightness is not set.
const BrightnessTestActivePercentDefault = 50

func validateBrightnessPercent(percent uint8) error {
	if percent > 100 {
		return fmt.Errorf("brightness must be between 0 and 100 percent, got %d", percent)
	}
	return nil
}

// BrightnessSetting is one output→percent assignment.
type BrightnessSetting struct {
	Output  uint8
	Percent uint8
	Label   string
}

var reBrightnessOutput = regexp.MustCompile(`(?i)^(?:O|FO|AUX)?(\d+)$`)

// ParseBrightnessArgs parses OUTPUT=PERCENT tokens such as "O1=50", "2=10",
// spread across multiple arguments and/or comma-separated within one argument.
// Later assignments for the same output override earlier ones.
func ParseBrightnessArgs(args []string) ([]BrightnessSetting, error) {
	tokens := splitListArgs(args)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no brightness assignments specified (expected O1=50,O2=5)")
	}

	settings := make([]BrightnessSetting, 0, len(tokens))
	seen := make(map[uint8]int, len(tokens))
	for _, tok := range tokens {
		parts := strings.SplitN(tok, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid brightness assignment %q (expected OUTPUT=PERCENT, e.g. O1=50)", tok)
		}
		output, label, err := parseBrightnessOutput(parts[0])
		if err != nil {
			return nil, err
		}
		percent, err := parseBrightnessPercent(parts[1])
		if err != nil {
			return nil, err
		}
		setting := BrightnessSetting{Output: output, Percent: percent, Label: label}
		if idx, ok := seen[output]; ok {
			settings[idx] = setting
			continue
		}
		seen[output] = len(settings)
		settings = append(settings, setting)
	}
	return settings, nil
}

func parseBrightnessOutput(s string) (uint8, string, error) {
	s = strings.TrimSpace(s)
	m := reBrightnessOutput.FindStringSubmatch(s)
	if m == nil {
		return 0, "", fmt.Errorf("invalid output %q (expected O1, FO2, AUX3 or a number)", s)
	}
	num, err := strconv.ParseUint(m[1], 10, 8)
	if err != nil {
		return 0, "", fmt.Errorf("invalid output %q: %w", s, err)
	}
	if num == 0 {
		return 0, "", fmt.Errorf("output numbers start at 1")
	}
	return uint8(num), fmt.Sprintf("O%d", num), nil
}

func parseBrightnessPercent(s string) (uint8, error) {
	s = strings.TrimSpace(s)
	v, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid brightness %q: %w", s, err)
	}
	if v > 100 {
		return 0, fmt.Errorf("brightness must be between 0 and 100 percent, got %d", v)
	}
	return uint8(v), nil
}

// RunBrightnessIdentifyTest saves brightness CVs, turns all outputs off, then lights
// one output at a time so the modeler can match physical lights to O/FO/AUX numbers.
// Original values are restored on return (including via defer on early exit).
func RunBrightnessIdentifyTest(decoder BrightnessChangeable, activePercent uint8, hooks BrightnessIdentifyHooks) ([]OutputBrightness, error) {
	if err := validateBrightnessPercent(activePercent); err != nil {
		return nil, fmt.Errorf("active brightness: %w", err)
	}

	snapshot, err := decoder.SnapshotBrightness()
	if err != nil {
		return nil, err
	}

	restoreSnapshot := func() error {
		logrus.Debugf("brightness test: restoring %d outputs from snapshot", len(snapshot))
		for _, s := range snapshot {
			logrus.Debugf("brightness test: restore output O%d cv%d=%d (was %d%%)", s.Output, s.CV, s.Value, s.Percent)
			if err := decoder.SetBrightnessRaw(s.Output, s.Value); err != nil {
				return fmt.Errorf("failed to restore output %d (cv%d): %w", s.Output, s.CV, err)
			}
		}
		return nil
	}
	defer func() { _ = restoreSnapshot() }()

	setAllOff := func() error {
		logrus.Debugf("brightness test: turning off all %d outputs", len(snapshot))
		for _, s := range snapshot {
			logrus.Debugf("brightness test: set output O%d to 0%%", s.Output)
			if err := decoder.SetBrightness(s.Output, 0); err != nil {
				return fmt.Errorf("failed to turn off output %d: %w", s.Output, err)
			}
		}
		return nil
	}

	setOnlyOn := func(output uint8) error {
		logrus.Debugf("brightness test: lighting only output O%d", output)
		if err := setAllOff(); err != nil {
			return err
		}
		logrus.Debugf("brightness test: set output O%d to %d%%", output, activePercent)
		if err := decoder.SetBrightness(output, activePercent); err != nil {
			return fmt.Errorf("failed to turn on output %d: %w", output, err)
		}
		return nil
	}

	if err := setAllOff(); err != nil {
		return snapshot, err
	}

	total := len(snapshot)
	for i, state := range snapshot {
		index := i + 1

		if err := setOnlyOn(state.Output); err != nil {
			return snapshot, err
		}

		if hooks.OnOutput != nil {
			if err := hooks.OnOutput(state, index, total); err != nil {
				return snapshot, err
			}
		}

		if hooks.WaitNext != nil {
			if err := hooks.WaitNext(state, index, total); err != nil {
				return snapshot, err
			}
		}
	}

	return snapshot, nil
}
