package cmd

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

type CommandStationRuntime struct {
	DccSync    CommandStationDccSyncPort
	SessionCtl CommandStationSessionPort
}

// CommandStation implements CRUD over the command-station catalogue.
type CommandStation struct {
	stations       *repo.CommandStations
	layoutStations *repo.LayoutCommandStations
	layouts        *repo.Layouts
	sec            security.CommandStationSecurityContext
	runtime        CommandStationRuntime
}

func NewCommandStation(
	stations *repo.CommandStations,
	layoutStations *repo.LayoutCommandStations,
	layouts *repo.Layouts,
) *CommandStation {
	return &CommandStation{
		stations:       stations,
		layoutStations: layoutStations,
		layouts:        layouts,
	}
}

func (s *CommandStation) SetRuntime(r CommandStationRuntime) {
	s.runtime = r
}

func (s *CommandStation) ListAll(ctx context.Context) ([]domain.CommandStation, error) {
	return s.stations.ListAll(ctx)
}

func (s *CommandStation) Get(ctx context.Context, id uint) (domain.CommandStation, error) {
	row, err := s.stations.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrCommandStationNotFound) {
			return domain.CommandStation{}, svcerrors.ErrCommandStationNotFound
		}
		return domain.CommandStation{}, err
	}
	return row, nil
}

type CommandStationCreateInput struct {
	Name             string
	Kind             domain.CommandStationKind
	ConnectionURI    string
	SpeedSteps       uint
	HeartbeatSecs    float64
	DeadmanSecs      float64
	PollIntervalMs   uint
	Z21ServerEnabled bool
	Z21IPStickiness  bool
	WithrottleServerEnabled bool
}

func (s *CommandStation) Create(ctx context.Context, eff domain.EffectiveRoles, in CommandStationCreateInput) (domain.CommandStation, error) {
	if err := s.checkCatalogManage(eff); err != nil {
		return domain.CommandStation{}, err
	}
	name, kind, uri, steps, err := validation.SanitiseCommandStationInput(in.Name, in.Kind, in.ConnectionURI, in.SpeedSteps)
	if err != nil {
		return domain.CommandStation{}, err
	}
	heartbeat, deadman, err := validation.SanitiseCommandStationTiming(in.HeartbeatSecs, in.DeadmanSecs)
	if err != nil {
		return domain.CommandStation{}, err
	}
	pollInterval, err := validation.SanitiseCommandStationPollInterval(in.PollIntervalMs)
	if err != nil {
		return domain.CommandStation{}, err
	}
	if _, err := s.stations.FindByName(ctx, name); err == nil {
		return domain.CommandStation{}, svcerrors.ErrCommandStationNameTaken
	} else if !errors.Is(err, repo.ErrCommandStationNotFound) {
		return domain.CommandStation{}, err
	}

	now := time.Now().UTC()
	row := domain.CommandStation{
		Name:             name,
		Kind:             kind,
		ConnectionURI:    uri,
		SpeedSteps:       steps,
		HeartbeatSecs:    heartbeat,
		DeadmanSecs:      deadman,
		PollIntervalMs:   pollInterval,
		Z21ServerEnabled: in.Z21ServerEnabled,
		Z21IPStickiness:  in.Z21IPStickiness,
		WithrottleServerEnabled: in.WithrottleServerEnabled,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.validateInboundPorts(ctx, row, 0); err != nil {
		return domain.CommandStation{}, err
	}
	if err := s.stations.Insert(ctx, &row); err != nil {
		return domain.CommandStation{}, err
	}
	return row, nil
}

type CommandStationUpdateInput struct {
	Name             *string
	Kind             *domain.CommandStationKind
	ConnectionURI    *string
	SpeedSteps       *uint
	HeartbeatSecs    *float64
	DeadmanSecs      *float64
	PollIntervalMs   *uint
	Z21ServerEnabled *bool
	Z21IPStickiness  *bool
	WithrottleServerEnabled *bool
}

func (s *CommandStation) Update(ctx context.Context, eff domain.EffectiveRoles, id uint, in CommandStationUpdateInput) (domain.CommandStation, error) {
	if err := s.checkCatalogManage(eff); err != nil {
		return domain.CommandStation{}, err
	}
	row, err := s.Get(ctx, id)
	if err != nil {
		return domain.CommandStation{}, err
	}

	if in.Name != nil {
		name, err := validation.SanitiseCommandStationName(*in.Name)
		if err != nil {
			return domain.CommandStation{}, err
		}
		if name != row.Name {
			if other, err := s.stations.FindByName(ctx, name); err == nil {
				if other.ID != row.ID {
					return domain.CommandStation{}, svcerrors.ErrCommandStationNameTaken
				}
			} else if !errors.Is(err, repo.ErrCommandStationNotFound) {
				return domain.CommandStation{}, err
			}
			row.Name = name
		}
	}
	if in.Kind != nil {
		if !in.Kind.IsValid() {
			return domain.CommandStation{}, svcerrors.ErrCommandStationKindInvalid
		}
		row.Kind = *in.Kind
	}
	if in.ConnectionURI != nil {
		row.ConnectionURI = strings.TrimSpace(*in.ConnectionURI)
	}
	if in.SpeedSteps != nil {
		steps, err := validation.SanitiseCommandStationSpeedSteps(*in.SpeedSteps)
		if err != nil {
			return domain.CommandStation{}, err
		}
		row.SpeedSteps = steps
	}
	heartbeat := row.HeartbeatSecs
	deadman := row.DeadmanSecs
	if in.HeartbeatSecs != nil {
		heartbeat = *in.HeartbeatSecs
	}
	if in.DeadmanSecs != nil {
		deadman = *in.DeadmanSecs
	}
	if in.HeartbeatSecs != nil || in.DeadmanSecs != nil {
		heartbeat, deadman, err = validation.SanitiseCommandStationTiming(heartbeat, deadman)
		if err != nil {
			return domain.CommandStation{}, err
		}
		row.HeartbeatSecs = heartbeat
		row.DeadmanSecs = deadman
	}
	if in.PollIntervalMs != nil {
		pollInterval, err := validation.SanitiseCommandStationPollInterval(*in.PollIntervalMs)
		if err != nil {
			return domain.CommandStation{}, err
		}
		row.PollIntervalMs = pollInterval
	}
	if in.Z21ServerEnabled != nil {
		row.Z21ServerEnabled = *in.Z21ServerEnabled
	}
	if in.Z21IPStickiness != nil {
		row.Z21IPStickiness = *in.Z21IPStickiness
	}
	if in.WithrottleServerEnabled != nil {
		row.WithrottleServerEnabled = *in.WithrottleServerEnabled
	}
	row.UpdatedAt = time.Now().UTC()

	if err := s.validateInboundPorts(ctx, row, row.ID); err != nil {
		return domain.CommandStation{}, err
	}
	if err := s.stations.Update(ctx, &row); err != nil {
		return domain.CommandStation{}, err
	}
	s.afterCatalogMutation(ctx, row)
	return row, nil
}

func (s *CommandStation) Delete(ctx context.Context, eff domain.EffectiveRoles, id uint) error {
	if err := s.checkCatalogManage(eff); err != nil {
		return err
	}
	row, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	atRisk, err := s.layoutStations.CountLayoutsWithOnlyStation(ctx, id, s.layouts)
	if err != nil {
		return err
	}
	if atRisk > 0 {
		return svcerrors.ErrLayoutNeedsAtLeastOneCommandStation
	}
	if err := s.layoutStations.DeleteAllForCommandStation(ctx, id); err != nil {
		return err
	}
	return s.stations.Delete(ctx, &row)
}

func (s *CommandStation) afterCatalogMutation(ctx context.Context, row domain.CommandStation) {
	if s.runtime.DccSync != nil {
		_ = s.runtime.DccSync.ObserveCommandStationCatalog(ctx, row.ID)
	}
	if s.runtime.SessionCtl != nil {
		s.runtime.SessionCtl.BroadcastCommandStationCatalogChanged(ctx, row)
	}
}

func (s *CommandStation) checkCatalogManage(eff domain.EffectiveRoles) error {
	decision := s.sec.CanManageCatalog(eff)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case security.ReasonForbidden:
		return svcerrors.ErrCommandStationForbidden
	default:
		return errors.New(decision.Reason)
	}
}

func (s *CommandStation) validateInboundPorts(ctx context.Context, row domain.CommandStation, excludeID uint) error {
	all, err := s.stations.ListAll(ctx)
	if err != nil {
		return err
	}
	if row.Z21ServerEnabled {
		port := row.EffectiveZ21InboundPort()
		for _, other := range all {
			if other.ID == excludeID || !other.Z21ServerEnabled {
				continue
			}
			if other.EffectiveZ21InboundPort() == port {
				return svcerrors.ErrCommandStationInboundPortConflict
			}
		}
	}
	if row.WithrottleServerEnabled {
		port := row.EffectiveWithrottleInboundPort()
		for _, other := range all {
			if other.ID == excludeID || !other.WithrottleServerEnabled {
				continue
			}
			if other.EffectiveWithrottleInboundPort() == port {
				return svcerrors.ErrCommandStationInboundPortConflict
			}
		}
	}
	return nil
}
