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

// VehicleTemplateFunctionSlot is one function on a template (list summary).
type VehicleTemplateFunctionSlot struct {
	Num      uint8
	Name     string
	Icon     domain.FunctionIcon
	Position int
}

// VehicleTemplateListEntry is a template row with owner login for list UIs.
type VehicleTemplateListEntry struct {
	domain.VehicleTemplate
	OwnerLogin string
	Functions  []VehicleTemplateFunctionSlot
}

// VehicleTemplateService manages the template catalogue.
type VehicleTemplateService struct {
	templates *repo.VehicleTemplates
	users     *repo.Users
	functions *repo.DccFunctions
}

// NewVehicleTemplateService constructs a VehicleTemplateService.
func NewVehicleTemplateService(
	t *repo.VehicleTemplates,
	u *repo.Users,
	f *repo.DccFunctions,
) *VehicleTemplateService {
	return &VehicleTemplateService{templates: t, users: u, functions: f}
}

// List returns every template with owner login.
func (s *VehicleTemplateService) List(ctx context.Context) ([]VehicleTemplateListEntry, error) {
	rows, err := s.templates.List(ctx)
	if err != nil {
		return nil, err
	}
	logins := make(map[uint]string)
	out := make([]VehicleTemplateListEntry, 0, len(rows))
	for _, row := range rows {
		login, ok := logins[row.OwnerUserID]
		if !ok {
			u, err := s.users.FindByID(ctx, row.OwnerUserID)
			if err != nil {
				login = "?"
			} else {
				login = u.Login
			}
			logins[row.OwnerUserID] = login
		}
		fns, err := s.functions.ListByTemplateID(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		slots := make([]VehicleTemplateFunctionSlot, 0, len(fns))
		for _, fn := range fns {
			slots = append(slots, VehicleTemplateFunctionSlot{
				Num:      fn.Num,
				Name:     fn.Name,
				Icon:     fn.Icon,
				Position: fn.Position,
			})
		}
		out = append(out, VehicleTemplateListEntry{
			VehicleTemplate: row,
			OwnerLogin:      login,
			Functions:       slots,
		})
	}
	return out, nil
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
