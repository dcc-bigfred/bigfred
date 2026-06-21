package protocol

// VehicleChangedPayload is the JSON body of layout.vehiclesChanged.
type VehicleChangedPayload struct {
	LayoutID  uint   `json:"layoutId"`
	Action    string `json:"action"` // "added" | "removed" | "updated"
	VehicleID string `json:"vehicleId"`
}

// TrainChangedPayload is the JSON body of layout.trainsChanged.
type TrainChangedPayload struct {
	LayoutID uint   `json:"layoutId"`
	Action   string `json:"action"` // "added" | "removed" | "updated"
	TrainID  string `json:"trainId"`
}
