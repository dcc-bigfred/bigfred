package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxRadioNoteLen = 80

// RadioPhrase is the closed walkie-talkie vocabulary (§4.4.1).
type RadioPhrase string

const (
	RadioStoppedAtSignal  RadioPhrase = "STOPPED_AT_SIGNAL_READY_TO_ENTER"
	RadioEntryPermitted   RadioPhrase = "ENTRY_PERMITTED"
	RadioCancelRoute      RadioPhrase = "CANCEL_ROUTE"
	RadioRouteSet         RadioPhrase = "ROUTE_SET"
	RadioAck              RadioPhrase = "ACK"
	RadioStopImmediately  RadioPhrase = "STOP_IMMEDIATELY"
	RadioReadyToDepart    RadioPhrase = "READY_TO_DEPART"
	RadioDepartureCleared RadioPhrase = "DEPARTURE_CLEARED"
)

// AllRadioPhrases lists every valid phrase in display order.
var AllRadioPhrases = []RadioPhrase{
	RadioStoppedAtSignal,
	RadioEntryPermitted,
	RadioCancelRoute,
	RadioRouteSet,
	RadioAck,
	RadioStopImmediately,
	RadioReadyToDepart,
	RadioDepartureCleared,
}

// IsValidRadioPhrase reports whether p is a known vocabulary entry.
func IsValidRadioPhrase(p RadioPhrase) bool {
	for _, known := range AllRadioPhrases {
		if p == known {
			return true
		}
	}
	return false
}

// RadioMessage is a single walkie-talkie message stored in Redis (§4.4.4).
type RadioMessage struct {
	ID               string
	LayoutID         uint
	FromUserID       uint
	FromLogin        string
	FromInterlockingID   *uint
	FromInterlockingName string
	ToUserID         *uint
	ToInterlockingID *uint

	ContextVehicleID *uint
	ContextTrainID   *uint
	ContextName      string

	Phrase RadioPhrase
	Note   string
	SentAt time.Time
}

var (
	ErrRadioInvalidTarget  = errors.New("radio_invalid_target")
	ErrRadioInvalidContext = errors.New("radio_invalid_context")
	ErrRadioInvalidPhrase  = errors.New("radio_invalid_phrase")
	ErrRadioNoteTooLong    = errors.New("radio_note_too_long")
)

// ValidateTarget enforces exactly-one on ToUserID / ToInterlockingID.
func ValidateTarget(toUserID, toInterlockingID uint) error {
	hasUser := toUserID != 0
	hasIlk := toInterlockingID != 0
	if hasUser == hasIlk {
		return ErrRadioInvalidTarget
	}
	return nil
}

// ValidateContext enforces exactly-one on ContextVehicleID / ContextTrainID.
func ValidateContext(vehicleID, trainID uint) error {
	hasVehicle := vehicleID != 0
	hasTrain := trainID != 0
	if hasVehicle == hasTrain {
		return ErrRadioInvalidContext
	}
	return nil
}

// ValidateNote trims and caps optional free text.
func ValidateNote(note string) (string, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > MaxRadioNoteLen {
		return "", ErrRadioNoteTooLong
	}
	return note, nil
}
