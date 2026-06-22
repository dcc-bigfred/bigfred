package cmd

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/helpers"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

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
	OwnerLogin        string
	OwnerOrganization string
	Functions         []VehicleTemplateFunctionSlot
}

// VehicleTemplateCreateInput is the payload for Create.
type VehicleTemplateCreateInput struct {
	OwnerUserID uint
	Name        string
	Description string
}

// VehicleTemplateUpdateInput is the payload for Update.
type VehicleTemplateUpdateInput struct {
	Name        string
	Description string
}

// VehicleTemplate manages the template catalogue (§4.1).
type VehicleTemplate struct {
	templates *repo.VehicleTemplates
	users     *repo.Users
	functions *repo.DccFunctions
	sec       security.FunctionSecurityContext
}

// NewVehicleTemplate constructs a VehicleTemplate use-case handler.
func NewVehicleTemplate(
	t *repo.VehicleTemplates,
	u *repo.Users,
	f *repo.DccFunctions,
) *VehicleTemplate {
	return &VehicleTemplate{templates: t, users: u, functions: f}
}

// List returns every template with owner login and function slots.
func (vt *VehicleTemplate) List(ctx context.Context) ([]VehicleTemplateListEntry, error) {
	rows, err := vt.templates.List(ctx)
	if err != nil {
		return nil, err
	}
	logins := make(map[uint]struct {
		login        string
		organization string
	})
	out := make([]VehicleTemplateListEntry, 0, len(rows))
	for _, row := range rows {
		info, ok := logins[row.OwnerUserID]
		if !ok {
			u, err := vt.users.FindByID(ctx, row.OwnerUserID)
			if err != nil {
				info.login = "?"
			} else {
				info.login = u.Login
				info.organization = u.Organization
			}
			logins[row.OwnerUserID] = info
		}
		entry, err := vt.entryFor(ctx, row, info.login, info.organization)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, nil
}

// Get loads a template by id.
func (vt *VehicleTemplate) Get(ctx context.Context, id uint) (domain.VehicleTemplate, error) {
	row, err := vt.templates.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleTemplateNotFound) {
			return domain.VehicleTemplate{}, svcerrors.ErrVehicleTemplateNotFound
		}
		return domain.VehicleTemplate{}, err
	}
	return row, nil
}

// Create registers a new template.
func (vt *VehicleTemplate) Create(ctx context.Context, in VehicleTemplateCreateInput) (domain.VehicleTemplate, error) {
	name, err := validation.SanitiseVehicleTemplateName(in.Name)
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
	if err := vt.templates.Insert(ctx, &row); err != nil {
		if helpers.IsUniqueViolation(err) {
			return domain.VehicleTemplate{}, svcerrors.ErrVehicleTemplateNameTaken
		}
		return domain.VehicleTemplate{}, err
	}
	return row, nil
}

// Update changes template name and description (owner or admin).
func (vt *VehicleTemplate) Update(
	ctx context.Context,
	actorID uint,
	eff domain.EffectiveRoles,
	id uint,
	in VehicleTemplateUpdateInput,
) (VehicleTemplateListEntry, error) {
	row, err := vt.Get(ctx, id)
	if err != nil {
		return VehicleTemplateListEntry{}, err
	}
	if d := vt.sec.CanEditTemplateFunctions(eff, actorID, row.OwnerUserID); !d.Allowed {
		return VehicleTemplateListEntry{}, svcerrors.ErrTemplateNotOwned
	}
	name, err := validation.SanitiseVehicleTemplateName(in.Name)
	if err != nil {
		return VehicleTemplateListEntry{}, err
	}
	row.Name = name
	row.Description = strings.TrimSpace(in.Description)
	row.Version++
	row.UpdatedAt = time.Now().UTC()
	if err := vt.templates.Update(ctx, &row); err != nil {
		if helpers.IsUniqueViolation(err) {
			return VehicleTemplateListEntry{}, svcerrors.ErrVehicleTemplateNameTaken
		}
		return VehicleTemplateListEntry{}, err
	}
	return vt.entryFor(ctx, row, "", "")
}

func (vt *VehicleTemplate) entryFor(
	ctx context.Context,
	row domain.VehicleTemplate,
	ownerLogin string,
	ownerOrganization string,
) (VehicleTemplateListEntry, error) {
	if ownerLogin == "" {
		u, err := vt.users.FindByID(ctx, row.OwnerUserID)
		if err != nil {
			ownerLogin = "?"
		} else {
			ownerLogin = u.Login
			ownerOrganization = u.Organization
		}
	}
	fns, err := vt.functions.ListByTemplateID(ctx, row.ID)
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
		VehicleTemplate:   row,
		OwnerLogin:        ownerLogin,
		OwnerOrganization: ownerOrganization,
		Functions:         slots,
	}, nil
}
