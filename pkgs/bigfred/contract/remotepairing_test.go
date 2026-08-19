package contract

import (
	"encoding/json"
	"testing"
)

func TestUnmarshalRemoteSessionAcceptsLuaEmptyObjects(t *testing.T) {
	raw := []byte(`{
		"protocol":"z21",
		"userId":9,
		"vehicleIds":{},
		"allowedAddrs":{},
		"allowAllVehicles":true,
		"pairedAt":1,
		"lastSeenAt":2,
		"clientKey":"z21:10.0.0.1:21105"
	}`)
	out, err := UnmarshalRemoteSession(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.VehicleIDs) != 0 {
		t.Fatalf("VehicleIDs=%v", out.VehicleIDs)
	}
	if len(out.AllowedAddrs) != 0 {
		t.Fatalf("AllowedAddrs=%v", out.AllowedAddrs)
	}
	if out.UserID != 9 || out.ClientKey != "z21:10.0.0.1:21105" {
		t.Fatalf("session=%+v", out)
	}
}

func TestUnmarshalRemoteSessionKeepsVehicleIDs(t *testing.T) {
	in := RemoteSessionWire{
		Protocol:     RemoteProtocolWithrottle,
		UserID:       3,
		VehicleIDs:   []string{"V-1", "V-2"},
		AllowedAddrs: []uint16{4, 5},
		ClientKey:    "withrottle:4242",
	}
	raw, err := MarshalRemoteSession(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := UnmarshalRemoteSession(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.VehicleIDs) != 2 || out.VehicleIDs[0] != "V-1" {
		t.Fatalf("VehicleIDs=%v", out.VehicleIDs)
	}
	if len(out.AllowedAddrs) != 2 || out.AllowedAddrs[1] != 5 {
		t.Fatalf("AllowedAddrs=%v", out.AllowedAddrs)
	}
}

func TestUnmarshalRemotePendingAcceptsLuaEmptyObjects(t *testing.T) {
	raw := []byte(`{"layoutId":1,"commandStationId":2,"protocol":"z21","userId":9,"vehicleIds":{},"allowedAddrs":{}}`)
	out, err := UnmarshalRemotePending(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.VehicleIDs) != 0 || len(out.AllowedAddrs) != 0 {
		t.Fatalf("pending=%+v", out)
	}
}

func TestUnmarshalRemoteSessionRejectsNonArrayVehicleIds(t *testing.T) {
	raw := []byte(`{"vehicleIds":{"oops":true}}`)
	if _, err := UnmarshalRemoteSession(raw); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestRemoteSessionWireJSONUsesUnmarshalJSON(t *testing.T) {
	var w RemoteSessionWire
	if err := json.Unmarshal([]byte(`{"vehicleIds":{},"userId":1}`), &w); err != nil {
		t.Fatal(err)
	}
	if w.UserID != 1 || w.VehicleIDs != nil && len(w.VehicleIDs) != 0 {
		t.Fatalf("wire=%+v", w)
	}
}
