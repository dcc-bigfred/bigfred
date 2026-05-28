package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
	"github.com/keskad/loco/pkgs/server/security"
)

var (
	ErrCommandStationNotFound     = errors.New("command_station_not_found")
	ErrCommandStationNameTaken    = errors.New("command_station_name_taken")
	ErrCommandStationNameRequired = errors.New("command_station_name_required")
	ErrCommandStationKindInvalid  = errors.New("command_station_kind_invalid")
	ErrCommandStationSpeedInvalid = errors.New("command_station_speed_steps_invalid")
	ErrCommandStationForbidden    = errors.New("forbidden")

	// ErrLayoutNeedsAtLeastOneCommandStation is returned when a layout
	// mutation would leave a non-system layout with zero attached
	// command stations (§4.1).
	ErrLayoutNeedsAtLeastOneCommandStation = errors.New("layout_needs_at_least_one_command_station")

	// ErrSystemLayoutCommandStationsImmutable is returned when POST /
	// PUT / DELETE on the system layout's command-station subresource
	// is attempted — the system layout's attachment set is virtual
	// (§4.1).
	ErrSystemLayoutCommandStationsImmutable = errors.New("default_layout_command_stations_immutable")
)

const maxCommandStationNameLen = 64

var validSpeedSteps = map[uint]struct{}{
	14: {}, 28: {}, 128: {},
}

// CommandStationService implements CRUD over the command-station
// catalogue.
type CommandStationService struct {
	stations       *repo.CommandStations
	layoutStations *repo.LayoutCommandStations
	layouts        *repo.Layouts
	sec            security.CommandStationSecurityContext
}

// NewCommandStationService constructs a CommandStationService.
func NewCommandStationService(
	stations *repo.CommandStations,
	layoutStations *repo.LayoutCommandStations,
	layouts *repo.Layouts,
) *CommandStationService {
	return &CommandStationService{
		stations:       stations,
		layoutStations: layoutStations,
		layouts:        layouts,
	}
}

// ListAll returns every command station in the catalogue (admin view).
func (s *CommandStationService) ListAll(ctx context.Context) ([]domain.CommandStation, error) {
	return s.stations.ListAll(ctx)
}

// Get looks a command station up by id.
func (s *CommandStationService) Get(ctx context.Context, id uint) (domain.CommandStation, error) {
	row, err := s.stations.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrCommandStationNotFound) {
			return domain.CommandStation{}, ErrCommandStationNotFound
		}
		return domain.CommandStation{}, err
	}
	return row, nil
}

// CommandStationCreateInput is the validated payload of Create.
type CommandStationCreateInput struct {
	Name          string
	Kind          domain.CommandStationKind
	ConnectionURI string
	SpeedSteps    uint
}

// Create inserts a new command station into the catalogue.
func (s *CommandStationService) Create(ctx context.Context, eff domain.EffectiveRoles, in CommandStationCreateInput) (domain.CommandStation, error) {
	if err := s.checkCatalogManage(eff); err != nil {
		return domain.CommandStation{}, err
	}
	name, kind, uri, steps, err := s.validateCatalogInput(in.Name, in.Kind, in.ConnectionURI, in.SpeedSteps)
	if err != nil {
		return domain.CommandStation{}, err
	}
	if _, err := s.stations.FindByName(ctx, name); err == nil {
		return domain.CommandStation{}, ErrCommandStationNameTaken
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

// CommandStationUpdateInput carries optional fields for Update.
type CommandStationUpdateInput struct {
	Name          *string
	Kind          *domain.CommandStationKind
	ConnectionURI *string
	SpeedSteps    *uint
}

// Update mutates an existing command station row.
func (s *CommandStationService) Update(ctx context.Context, eff domain.EffectiveRoles, id uint, in CommandStationUpdateInput) (domain.CommandStation, error) {
	if err := s.checkCatalogManage(eff); err != nil {
		return domain.CommandStation{}, err
	}
	row, err := s.Get(ctx, id)
	if err != nil {
		return domain.CommandStation{}, err
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || len(name) > maxCommandStationNameLen {
			return domain.CommandStation{}, ErrCommandStationNameRequired
		}
		if name != row.Name {
			if other, err := s.stations.FindByName(ctx, name); err == nil {
				if other.ID != row.ID {
					return domain.CommandStation{}, ErrCommandStationNameTaken
				}
			} else if !errors.Is(err, repo.ErrCommandStationNotFound) {
				return domain.CommandStation{}, err
			}
			row.Name = name
		}
	}
	if in.Kind != nil {
		if !in.Kind.IsValid() {
			return domain.CommandStation{}, ErrCommandStationKindInvalid
		}
		row.Kind = *in.Kind
	}
	if in.ConnectionURI != nil {
		row.ConnectionURI = strings.TrimSpace(*in.ConnectionURI)
	}
	if in.SpeedSteps != nil {
		if _, ok := validSpeedSteps[*in.SpeedSteps]; !ok {
			return domain.CommandStation{}, ErrCommandStationSpeedInvalid
		}
		row.SpeedSteps = *in.SpeedSteps
	}
	row.UpdatedAt = time.Now().UTC()

	if err := s.stations.Update(ctx, &row); err != nil {
		return domain.CommandStation{}, err
	}
	return row, nil
}

// Delete removes a command station when no non-system layout would be
// left without one (§4.1). Cascades join rows.
func (s *CommandStationService) Delete(ctx context.Context, eff domain.EffectiveRoles, id uint) error {
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
		return ErrLayoutNeedsAtLeastOneCommandStation
	}
	if err := s.layoutStations.DeleteAllForCommandStation(ctx, id); err != nil {
		return err
	}
	return s.stations.Delete(ctx, &row)
}

func (s *CommandStationService) validateCatalogInput(name string, kind domain.CommandStationKind, uri string, speedSteps uint) (string, domain.CommandStationKind, string, uint, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxCommandStationNameLen {
		return "", "", "", 0, ErrCommandStationNameRequired
	}
	if !kind.IsValid() {
		return "", "", "", 0, ErrCommandStationKindInvalid
	}
	if speedSteps == 0 {
		speedSteps = 128
	}
	if _, ok := validSpeedSteps[speedSteps]; !ok {
		return "", "", "", 0, ErrCommandStationSpeedInvalid
	}
	return name, kind, strings.TrimSpace(uri), speedSteps, nil
}

func (s *CommandStationService) checkCatalogManage(eff domain.EffectiveRoles) error {
	decision := s.sec.CanManageCatalog(eff)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case "forbidden":
		return ErrCommandStationForbidden
	default:
		return errors.New(decision.Reason)
	}
}
