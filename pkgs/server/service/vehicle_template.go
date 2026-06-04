package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
)

var (
	ErrVehicleTemplateNotFound = errors.New("vehicle_template_not_found")
	ErrVehicleTemplateNameRequired = errors.New("vehicle_template_name_required")
	ErrVehicleTemplateNameTaken    = errors.New("vehicle_template_name_taken")
)

const maxVehicleTemplateNameLen = 64

// VehicleTemplateService manages the template catalogue.
type VehicleTemplateService struct {
	templates *repo.VehicleTemplates
}

// NewVehicleTemplateService constructs a VehicleTemplateService.
func NewVehicleTemplateService(t *repo.VehicleTemplates) *VehicleTemplateService {
	return &VehicleTemplateService{templates: t}
}

// List returns every template.
func (s *VehicleTemplateService) List(ctx context.Context) ([]domain.VehicleTemplate, error) {
	return s.templates.List(ctx)
}

// Get loads a template by id.
func (s *VehicleTemplateService) Get(ctx context.Context, id uint) (domain.VehicleTemplate, error) {
	row, err := s.templates.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleTemplateNotFound) {
			return domain.VehicleTemplate{}, ErrVehicleTemplateNotFound
		}
		return domain.VehicleTemplate{}, err
	}
	return row, nil
}

// VehicleTemplateCreateInput is the payload for Create.
type VehicleTemplateCreateInput struct {
	OwnerUserID uint
	Name        string
	Description string
}

// Create registers a new template.
func (s *VehicleTemplateService) Create(ctx context.Context, in VehicleTemplateCreateInput) (domain.VehicleTemplate, error) {
	name, err := sanitiseTemplateName(in.Name)
	if err != nil {
		return domain.VehicleTemplate{}, err
	}
	now := time.Now().UTC()
	row := domain.VehicleTemplate{
		Name:        name,
		Description: strings.TrimSpace(in.Description),
		OwnerUserID: in.OwnerUserID,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.templates.Insert(ctx, &row); err != nil {
		if isUniqueViolation(err) {
			return domain.VehicleTemplate{}, ErrVehicleTemplateNameTaken
		}
		return domain.VehicleTemplate{}, err
	}
	return row, nil
}

func sanitiseTemplateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", ErrVehicleTemplateNameRequired
	}
	if len(name) > maxVehicleTemplateNameLen {
		name = name[:maxVehicleTemplateNameLen]
	}
	return name, nil
}

// isUniqueViolation is a best-effort SQLite unique constraint detector.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed")
}
