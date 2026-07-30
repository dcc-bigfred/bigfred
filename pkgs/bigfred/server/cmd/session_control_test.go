package cmd_test

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

type fakeControlSession struct {
	sessionID      string
	userID         uint
	login          string
	organization   string
	layoutID       uint
	commandStation uint
}

func (s *fakeControlSession) SessionID() string           { return s.sessionID }
func (s *fakeControlSession) UserID() uint                { return s.userID }
func (s *fakeControlSession) Login() string               { return s.login }
func (s *fakeControlSession) Organization() string        { return s.organization }
func (s *fakeControlSession) LayoutID() uint              { return s.layoutID }
func (s *fakeControlSession) CurrentCommandStation() uint { return s.commandStation }
func (s *fakeControlSession) SetCommandStation(commandStationID uint) uint {
	s.commandStation = commandStationID
	return commandStationID
}

type recordedTyped struct {
	eventType string
	payload   any
}

type fakeControlClient struct {
	session *fakeControlSession
	sent    []recordedTyped
	acks    []struct {
		id   string
		ok   bool
		code string
	}
}

func (c *fakeControlClient) Session() cmd.ControlSession { return c.session }
func (c *fakeControlClient) SendTyped(eventType string, payload any) {
	c.sent = append(c.sent, recordedTyped{eventType: eventType, payload: payload})
}
func (c *fakeControlClient) SendAck(requestID string, ok bool, errCode string) {
	c.acks = append(c.acks, struct {
		id   string
		ok   bool
		code string
	}{id: requestID, ok: ok, code: errCode})
}

func TestBroadcastLayoutAvailableCommandStations(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	layoutSvc := freshLayoutSvc(t, ctx, bundle)
	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)

	cs1 := insertCommandStation(t, ctx, bundle.CommandStations, "Z21 A")
	cs2 := insertCommandStation(t, ctx, bundle.CommandStations, "Z21 B")

	layout, err := layoutSvc.Create(ctx, adminEff, cmd.LayoutCreateInput{
		Name:              "Ops",
		CreatedBy:         1,
		CommandStationIDs: []uint{cs1.ID},
		AdminPIN:          "1234",
	})
	if err != nil {
		t.Fatalf("create layout: %v", err)
	}
	other, err := layoutSvc.Create(ctx, adminEff, cmd.LayoutCreateInput{
		Name:              "Other",
		CreatedBy:         1,
		CommandStationIDs: []uint{cs1.ID},
		AdminPIN:          "5678",
	})
	if err != nil {
		t.Fatalf("create other layout: %v", err)
	}

	ctl := cmd.NewSessionControl(cmd.SessionControlConfig{
		CommandStns: bundle.CommandStations,
		LayoutCS:    bundle.LayoutCommandStations,
		Layouts:     bundle.Layouts,
	})

	target := &fakeControlClient{session: &fakeControlSession{
		sessionID: "s1",
		userID:    1,
		layoutID:  layout.ID,
	}}
	otherClient := &fakeControlClient{session: &fakeControlSession{
		sessionID: "s2",
		userID:    2,
		layoutID:  other.ID,
	}}
	ctl.HandleOpened(ctx, target)
	ctl.HandleOpened(ctx, otherClient)
	target.sent = nil
	otherClient.sent = nil

	if _, err := layoutSvc.SetCommandStations(ctx, adminEff, layout.ID, 1, []uint{cs1.ID, cs2.ID}); err != nil {
		t.Fatalf("set command stations: %v", err)
	}
	ctl.BroadcastLayoutAvailableCommandStations(ctx, layout.ID)

	if len(otherClient.sent) != 0 {
		t.Fatalf("other layout session should not receive broadcast, got %#v", otherClient.sent)
	}
	if len(target.sent) != 1 {
		t.Fatalf("expected one broadcast to target session, got %#v", target.sent)
	}
	if target.sent[0].eventType != cmd.TypeSessionAvailableCommandStationsChanged {
		t.Fatalf("unexpected event type %q", target.sent[0].eventType)
	}
	payload, ok := target.sent[0].payload.(cmd.AvailableCommandStationsChangedPayload)
	if !ok {
		t.Fatalf("unexpected payload type %T", target.sent[0].payload)
	}
	if len(payload.AvailableCommandStations) != 2 {
		t.Fatalf("expected 2 stations after attach, got %d", len(payload.AvailableCommandStations))
	}
	ids := map[uint]struct{}{}
	for _, st := range payload.AvailableCommandStations {
		ids[st.ID] = struct{}{}
	}
	if _, ok := ids[cs1.ID]; !ok {
		t.Fatalf("missing cs1 %d in payload", cs1.ID)
	}
	if _, ok := ids[cs2.ID]; !ok {
		t.Fatalf("missing cs2 %d in payload", cs2.ID)
	}

	target.sent = nil
	if _, err := layoutSvc.SetCommandStations(ctx, adminEff, layout.ID, 1, []uint{cs2.ID}); err != nil {
		t.Fatalf("set command stations remove: %v", err)
	}
	ctl.BroadcastLayoutAvailableCommandStations(ctx, layout.ID)
	if len(target.sent) != 1 {
		t.Fatalf("expected one broadcast after remove, got %#v", target.sent)
	}
	payload, ok = target.sent[0].payload.(cmd.AvailableCommandStationsChangedPayload)
	if !ok {
		t.Fatalf("unexpected payload type %T", target.sent[0].payload)
	}
	if len(payload.AvailableCommandStations) != 1 || payload.AvailableCommandStations[0].ID != cs2.ID {
		t.Fatalf("expected only cs2 after remove, got %#v", payload.AvailableCommandStations)
	}
}
