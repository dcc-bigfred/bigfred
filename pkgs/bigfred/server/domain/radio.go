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
	RadioAck                                  RadioPhrase = "ACK"
	RadioAgreed                               RadioPhrase = "AGREED"
	RadioStoppedAtSignal                      RadioPhrase = "STOPPED_AT_SIGNAL_READY_TO_ENTER"
	RadioReadyToDepart                        RadioPhrase = "READY_TO_DEPART"
	RadioAcceptedDepartureOnReplacementSignal RadioPhrase = "ACCEPTED_DEPARTURE_ON_REPLACEMENT_SIGNAL"
	RadioAcceptedHelperDetachAtStation        RadioPhrase = "ACCEPTED_HELPER_DETACH_AT_STATION"
	RadioAcceptedWaitingForOppositeTrain      RadioPhrase = "ACCEPTED_WAITING_FOR_OPPOSITE_TRAIN"
	RadioTrainArrivedCompleteAtStation        RadioPhrase = "TRAIN_ARRIVED_COMPLETE_AT_STATION"
	RadioLocoReadyForRunAround                RadioPhrase = "LOCO_READY_FOR_RUN_AROUND"
	RadioAcceptedCrossingsExtraCaution        RadioPhrase = "ACCEPTED_CROSSINGS_EXTRA_CAUTION"
	RadioAcceptedPushingBeyondPoints          RadioPhrase = "ACCEPTED_PUSHING_BEYOND_POINTS"
	RadioAcceptedStoppingShunting             RadioPhrase = "ACCEPTED_STOPPING_SHUNTING"
	RadioPassedPointsAheadWaitingRunAroundRoute RadioPhrase = "PASSED_POINTS_AHEAD_WAITING_RUN_AROUND_ROUTE"
	RadioReachedPointsRearWaitingRunAroundRoute RadioPhrase = "REACHED_POINTS_REAR_WAITING_RUN_AROUND_ROUTE"
	RadioReadyToCoupleWagons                    RadioPhrase = "READY_TO_COUPLE_WAGONS"
	RadioReadyToUncoupleWagons                  RadioPhrase = "READY_TO_UNCOUPLE_WAGONS"
	RadioReportingConsistCoupledToLoco          RadioPhrase = "REPORTING_CONSIST_COUPLED_TO_LOCO"
	RadioWagonsTaken                            RadioPhrase = "WAGONS_TAKEN"
	RadioWagonsSetAside                         RadioPhrase = "WAGONS_SET_ASIDE"
	RadioLinkRestored                         RadioPhrase = "RADIO_LINK_RESTORED"
	RadioLevelCrossingGatesOpen               RadioPhrase = "LEVEL_CROSSING_GATES_OPEN"

	RadioReportAcknowledged                      RadioPhrase = "REPORT_ACKNOWLEDGED"
	RadioRepetitionCorrect                       RadioPhrase = "REPETITION_CORRECT"
	RadioConfirmed                               RadioPhrase = "CONFIRMED"
	RadioNo                                      RadioPhrase = "NO"
	RadioRefusedWaitForSignal                    RadioPhrase = "REFUSED_WAIT_FOR_SIGNAL"
	RadioEntryPermitted                          RadioPhrase = "ENTRY_PERMITTED"
	RadioDepartureCleared                        RadioPhrase = "DEPARTURE_CLEARED"
	RadioDepartureOnReplacementSignal            RadioPhrase = "DEPARTURE_ON_REPLACEMENT_SIGNAL"
	RadioRouteSet                                RadioPhrase = "ROUTE_SET"
	RadioArrivalCompleteAcknowledged             RadioPhrase = "ARRIVAL_COMPLETE_ACKNOWLEDGED"
	RadioTrainTrack1FreeReceiveTrack1            RadioPhrase = "TRAIN_TRACK_1_FREE_RECEIVE_TRACK_1"
	RadioTrainTrack2FreeReceiveTrack2            RadioPhrase = "TRAIN_TRACK_2_FREE_RECEIVE_TRACK_2"
	RadioTrainTrack3FreeReceiveTrack3            RadioPhrase = "TRAIN_TRACK_3_FREE_RECEIVE_TRACK_3"
	RadioTrainTrack4FreeReceiveTrack4            RadioPhrase = "TRAIN_TRACK_4_FREE_RECEIVE_TRACK_4"
	RadioTrainTrack5FreeReceiveTrack5            RadioPhrase = "TRAIN_TRACK_5_FREE_RECEIVE_TRACK_5"
	RadioTrainTrack6FreeReceiveTrack6            RadioPhrase = "TRAIN_TRACK_6_FREE_RECEIVE_TRACK_6"
	RadioTrainTrack7FreeReceiveTrack7            RadioPhrase = "TRAIN_TRACK_7_FREE_RECEIVE_TRACK_7"
	RadioTrainTrack8FreeReceiveTrack8            RadioPhrase = "TRAIN_TRACK_8_FREE_RECEIVE_TRACK_8"
	RadioWrongRoadFromPostToStation              RadioPhrase = "WRONG_ROAD_FROM_POST_TO_STATION"
	RadioAcceptedWrongRoadFromPostToStation      RadioPhrase = "ACCEPTED_WRONG_ROAD_FROM_POST_TO_STATION"
	RadioCancelRoute                             RadioPhrase = "CANCEL_ROUTE"
	RadioShuntingExtraCautionThroughPoints       RadioPhrase = "SHUNTING_EXTRA_CAUTION_THROUGH_POINTS"
	RadioRunAroundPermitted                      RadioPhrase = "RUN_AROUND_PERMITTED"
	RadioPushingBeyondPointsPermitted            RadioPhrase = "PUSHING_BEYOND_POINTS_PERMITTED"
	RadioStopShuntingImmediately                 RadioPhrase = "STOP_SHUNTING_IMMEDIATELY"
	RadioHelperLocoWillDetachAtStation           RadioPhrase = "HELPER_LOCO_WILL_DETACH_AT_STATION"
	RadioStopImmediately                         RadioPhrase = "STOP_IMMEDIATELY"
	RadioAcceptedNotifyingGatekeeperAndNeighbors RadioPhrase = "ACCEPTED_NOTIFYING_GATEKEEPER_AND_NEIGHBORS"
	RadioTrainWaitingForOpposite                 RadioPhrase = "TRAIN_WAITING_FOR_OPPOSITE"
	RadioStaffOnTrackCautionSignal               RadioPhrase = "STAFF_ON_TRACK_CAUTION_SIGNAL"
)

// AllRadioPhrases lists every valid phrase (union of driver + signalman vocabularies).
var AllRadioPhrases = []RadioPhrase{
	RadioAck,
	RadioAgreed,
	RadioStoppedAtSignal,
	RadioReadyToDepart,
	RadioAcceptedDepartureOnReplacementSignal,
	RadioAcceptedHelperDetachAtStation,
	RadioAcceptedWaitingForOppositeTrain,
	RadioTrainArrivedCompleteAtStation,
	RadioLocoReadyForRunAround,
	RadioAcceptedCrossingsExtraCaution,
	RadioAcceptedPushingBeyondPoints,
	RadioAcceptedStoppingShunting,
	RadioPassedPointsAheadWaitingRunAroundRoute,
	RadioReachedPointsRearWaitingRunAroundRoute,
	RadioReadyToCoupleWagons,
	RadioReadyToUncoupleWagons,
	RadioReportingConsistCoupledToLoco,
	RadioWagonsTaken,
	RadioWagonsSetAside,
	RadioLinkRestored,
	RadioLevelCrossingGatesOpen,
	RadioReportAcknowledged,
	RadioRepetitionCorrect,
	RadioConfirmed,
	RadioNo,
	RadioRefusedWaitForSignal,
	RadioEntryPermitted,
	RadioDepartureCleared,
	RadioDepartureOnReplacementSignal,
	RadioRouteSet,
	RadioArrivalCompleteAcknowledged,
	RadioTrainTrack1FreeReceiveTrack1,
	RadioTrainTrack2FreeReceiveTrack2,
	RadioTrainTrack3FreeReceiveTrack3,
	RadioTrainTrack4FreeReceiveTrack4,
	RadioTrainTrack5FreeReceiveTrack5,
	RadioTrainTrack6FreeReceiveTrack6,
	RadioTrainTrack7FreeReceiveTrack7,
	RadioTrainTrack8FreeReceiveTrack8,
	RadioWrongRoadFromPostToStation,
	RadioAcceptedWrongRoadFromPostToStation,
	RadioCancelRoute,
	RadioShuntingExtraCautionThroughPoints,
	RadioRunAroundPermitted,
	RadioPushingBeyondPointsPermitted,
	RadioStopShuntingImmediately,
	RadioHelperLocoWillDetachAtStation,
	RadioStopImmediately,
	RadioAcceptedNotifyingGatekeeperAndNeighbors,
	RadioTrainWaitingForOpposite,
	RadioStaffOnTrackCautionSignal,
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
	ID                   string
	LayoutID             uint
	FromUserID           uint
	FromLogin            string
	FromInterlockingID   *uint
	FromInterlockingName string
	ToUserID             *uint
	ToInterlockingID     *uint

	ContextVehicleID *VehicleID
	ContextTrainID   *TrainID
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
func ValidateContext(vehicleID VehicleID, trainID TrainID) error {
	hasVehicle := !vehicleID.IsZero()
	hasTrain := !trainID.IsZero()
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
