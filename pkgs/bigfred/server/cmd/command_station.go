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
	Name          string
	Kind          domain.CommandStationKind
	ConnectionURI string
	SpeedSteps    uint
}

func (s *CommandStation) Create(ctx context.Context, eff domain.EffectiveRoles, in CommandStationCreateInput) (domain.CommandStation, error) {
	if err := s.checkCatalogManage(eff); err != nil {
		return domain.CommandStation{}, err
	}
	name, kind, uri, steps, err := validation.SanitiseCommandStationInput(in.Name, in.Kind, in.ConnectionURI, in.SpeedSteps)
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
		Name:          name,
		Kind:          kind,
		ConnectionURI: uri,
		SpeedSteps:    steps,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.stations.Insert(ctx, &row); err != nil {
		return domain.CommandStation{}, err
	}
	return row, nil
}

type CommandStationUpdateInput struct {
	Name          *string
	Kind          *domain.CommandStationKind
	ConnectionURI *string
	SpeedSteps    *uint
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
		if err := validation.ValidateCommandStationSpeedSteps(*in.SpeedSteps); err != nil {
			return domain.CommandStation{}, err
		}
		row.SpeedSteps = *in.SpeedSteps
	}
	row.UpdatedAt = time.Now().UTC()

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
