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
		entry, err := s.entryFor(ctx, row, login)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
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

// VehicleTemplateUpdateInput is the payload for Update.
type VehicleTemplateUpdateInput struct {
	Name        string
	Description string
}

// Update changes template name and description (owner or admin).
func (s *VehicleTemplateService) Update(
	ctx context.Context,
	actorID uint,
	eff domain.EffectiveRoles,
	id uint,
	in VehicleTemplateUpdateInput,
) (VehicleTemplateListEntry, error) {
	row, err := s.Get(ctx, id)
	if err != nil {
		return VehicleTemplateListEntry{}, err
	}
	sec := security.FunctionSecurityContext{}
	if d := sec.CanEditTemplateFunctions(eff, actorID, row.OwnerUserID); !d.Allowed {
		return VehicleTemplateListEntry{}, ErrTemplateNotOwned
	}
	name, err := sanitiseTemplateName(in.Name)
	if err != nil {
		return VehicleTemplateListEntry{}, err
	}
	row.Name = name
	row.Description = strings.TrimSpace(in.Description)
	row.Version++
	row.UpdatedAt = time.Now().UTC()
	if err := s.templates.Update(ctx, &row); err != nil {
		if isUniqueViolation(err) {
			return VehicleTemplateListEntry{}, ErrVehicleTemplateNameTaken
		}
		return VehicleTemplateListEntry{}, err
	}
	return s.entryFor(ctx, row, "")
}

func (s *VehicleTemplateService) entryFor(
	ctx context.Context,
	row domain.VehicleTemplate,
	ownerLogin string,
) (VehicleTemplateListEntry, error) {
	if ownerLogin == "" {
		u, err := s.users.FindByID(ctx, row.OwnerUserID)
		if err != nil {
			ownerLogin = "?"
		} else {
			ownerLogin = u.Login
		}
	}
	fns, err := s.functions.ListByTemplateID(ctx, row.ID)
	if err != nil {
		return VehicleTemplateListEntry{}, err
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
	return VehicleTemplateListEntry{
		VehicleTemplate: row,
		OwnerLogin:      ownerLogin,
		Functions:       slots,
	}, nil
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
