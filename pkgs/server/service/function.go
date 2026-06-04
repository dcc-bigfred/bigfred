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
	ErrFunctionNumInvalid      = errors.New("function_num_invalid")
	ErrFunctionIconInvalid     = errors.New("function_icon_invalid")
	ErrFunctionNameRequired    = errors.New("function_name_required")
	ErrFunctionNumTaken        = errors.New("function_num_taken")
	ErrFunctionNotFound        = errors.New("function_not_found")
	ErrOnlyOwnerCanEdit            = errors.New("only_owner_can_edit")
	ErrTemplateNotOwned            = errors.New("template_not_owned")
	ErrFunctionReplaceSourceInvalid = errors.New("function_replace_source_invalid")
)

const maxFunctionNameLen = 64

// ResolvedFunction is the effective slot a throttle or editor displays.
type ResolvedFunction struct {
	Num      uint8
	Name     string
	Icon     domain.FunctionIcon
	Position int
	Source   string // "template" | "vehicle"
}

// FunctionUpsertInput is the validated payload for upserting one slot.
type FunctionUpsertInput struct {
	Name     string
	Icon     domain.FunctionIcon
	Position int
}

// FunctionReorderEntry maps a function number to its display position.
type FunctionReorderEntry struct {
	Num      uint8
	Position int
}

// VehicleFunctionCatalogueEntry is one vehicle with its resolved function list.
type VehicleFunctionCatalogueEntry struct {
	VehicleID   uint
	VehicleName string
	OwnerID     uint
	OwnerLogin  string
	DCCAddress  *uint16
	Kind        domain.VehicleKind
	Functions   []ResolvedFunction
}

// FunctionService manages dcc_functions for vehicles and templates.
type FunctionService struct {
	functions *repo.DccFunctions
	vehicles  *repo.Vehicles
	templates *repo.VehicleTemplates
	users     *repo.Users
	sec       security.FunctionSecurityContext
}

// NewFunctionService constructs a FunctionService.
func NewFunctionService(
	f *repo.DccFunctions,
	v *repo.Vehicles,
	t *repo.VehicleTemplates,
	u *repo.Users,
) *FunctionService {
	return &FunctionService{functions: f, vehicles: v, templates: t, users: u}
}

// ListIcons returns the closed icon catalogue.
func (s *FunctionService) ListIcons() []domain.FunctionIcon {
	return domain.FunctionIcons()
}

// ListForVehicle returns the resolved function list for a vehicle.
func (s *FunctionService) ListForVehicle(ctx context.Context, vehicleID uint) ([]ResolvedFunction, error) {
	v, err := s.loadVehicle(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	if v.TemplateID != nil && v.FunctionsDetachedAt == nil {
		rows, err := s.functions.ListByTemplateID(ctx, *v.TemplateID)
		if err != nil {
			return nil, err
		}
		return toResolved(rows, "template"), nil
	}
	rows, err := s.functions.ListByVehicleID(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	return toResolved(rows, "vehicle"), nil
}

// ListFunctionCatalogue returns every vehicle with at least one function.
func (s *FunctionService) ListFunctionCatalogue(ctx context.Context) ([]VehicleFunctionCatalogueEntry, error) {
	vehicles, err := s.vehicles.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	logins := make(map[uint]string)
	out := make([]VehicleFunctionCatalogueEntry, 0, len(vehicles))
	for _, v := range vehicles {
		fns, err := s.ListForVehicle(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		if len(fns) == 0 {
			continue
		}
		login, ok := logins[v.OwnerUserID]
		if !ok {
			u, err := s.users.FindByID(ctx, v.OwnerUserID)
			if err != nil {
				login = "?"
			} else {
				login = u.Login
			}
			logins[v.OwnerUserID] = login
		}
		out = append(out, VehicleFunctionCatalogueEntry{
			VehicleID:   v.ID,
			VehicleName: v.Name,
			OwnerID:     v.OwnerUserID,
			OwnerLogin:  login,
			DCCAddress:  v.DCCAddress,
			Kind:        v.Kind,
			Functions:   fns,
		})
	}
	return out, nil
}

// ListForTemplate returns function rows owned by a template.
func (s *FunctionService) ListForTemplate(ctx context.Context, templateID uint) ([]ResolvedFunction, error) {
	if _, err := s.loadTemplate(ctx, templateID); err != nil {
		return nil, err
	}
	rows, err := s.functions.ListByTemplateID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	return toResolved(rows, "template"), nil
}

// UpsertVehicleSlot adds or updates one function on a vehicle (owner only).
func (s *FunctionService) UpsertVehicleSlot(
	ctx context.Context,
	actorID uint,
	vehicleID uint,
	num uint8,
	in FunctionUpsertInput,
) (domain.DccFunction, error) {
	if !domain.ValidFunctionNum(num) {
		return domain.DccFunction{}, ErrFunctionNumInvalid
	}
	v, err := s.loadVehicle(ctx, vehicleID)
	if err != nil {
		return domain.DccFunction{}, err
	}
	if d := s.sec.CanEditVehicleFunctions(actorID, v.OwnerUserID); !d.Allowed {
		return domain.DccFunction{}, ErrOnlyOwnerCanEdit
	}
	payload, err := validateFunctionInput(in)
	if err != nil {
		return domain.DccFunction{}, err
	}

	var out domain.DccFunction
	err = s.functions.Transaction(ctx, func(ctx context.Context) error {
		if err := s.ensureDetached(ctx, &v); err != nil {
			return err
		}
		row, err := s.upsertVehicleRow(ctx, v.ID, num, payload)
		if err != nil {
			return err
		}
		out = row
		return nil
	})
	return out, err
}

// DeleteVehicleSlot removes one function from a vehicle (owner only).
func (s *FunctionService) DeleteVehicleSlot(
	ctx context.Context,
	actorID, vehicleID uint,
	num uint8,
) error {
	if !domain.ValidFunctionNum(num) {
		return ErrFunctionNumInvalid
	}
	v, err := s.loadVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	if d := s.sec.CanEditVehicleFunctions(actorID, v.OwnerUserID); !d.Allowed {
		return ErrOnlyOwnerCanEdit
	}
	return s.functions.Transaction(ctx, func(ctx context.Context) error {
		if err := s.ensureDetached(ctx, &v); err != nil {
			return err
		}
		row, err := s.functions.FindByVehicleAndNum(ctx, v.ID, num)
		if err != nil {
			if errors.Is(err, repo.ErrDccFunctionNotFound) {
				return ErrFunctionNotFound
			}
			return err
		}
		return s.functions.Delete(ctx, &row)
	})
}

// ReorderVehicleSlots updates display order on a vehicle (owner only).
func (s *FunctionService) ReorderVehicleSlots(
	ctx context.Context,
	actorID, vehicleID uint,
	positions []FunctionReorderEntry,
) error {
	v, err := s.loadVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	if d := s.sec.CanEditVehicleFunctions(actorID, v.OwnerUserID); !d.Allowed {
		return ErrOnlyOwnerCanEdit
	}
	return s.functions.Transaction(ctx, func(ctx context.Context) error {
		if err := s.ensureDetached(ctx, &v); err != nil {
			return err
		}
		return s.applyReorder(ctx, func(ctx context.Context, num uint8) (domain.DccFunction, error) {
			return s.functions.FindByVehicleAndNum(ctx, v.ID, num)
		}, positions)
	})
}

// AttachVehicleToTemplate drops all vehicle-owned function rows and links the
// vehicle to the template so its list is read live from the template (§3a.6).
func (s *FunctionService) AttachVehicleToTemplate(
	ctx context.Context,
	actorID, vehicleID, templateID uint,
) ([]ResolvedFunction, error) {
	if templateID == 0 {
		return nil, ErrVehicleTemplateNotFound
	}
	v, err := s.loadVehicle(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	if d := s.sec.CanEditVehicleFunctions(actorID, v.OwnerUserID); !d.Allowed {
		return nil, ErrOnlyOwnerCanEdit
	}
	if _, err := s.loadTemplate(ctx, templateID); err != nil {
		return nil, err
	}
	err = s.functions.Transaction(ctx, func(ctx context.Context) error {
		if err := s.functions.DeleteAllByVehicleID(ctx, v.ID); err != nil {
			return err
		}
		tid := templateID
		v.TemplateID = &tid
		v.FunctionsDetachedAt = nil
		v.UpdatedAt = time.Now().UTC()
		return s.vehicles.Update(ctx, &v)
	})
	if err != nil {
		return nil, err
	}
	return s.ListForVehicle(ctx, vehicleID)
}

// CopyVehicleFunctionsFromVehicle replaces the target's function list with a
// snapshot of the source vehicle's effective list (owned rows, no template link).
func (s *FunctionService) CopyVehicleFunctionsFromVehicle(
	ctx context.Context,
	actorID, targetVehicleID, sourceVehicleID uint,
) ([]ResolvedFunction, error) {
	if sourceVehicleID == 0 || sourceVehicleID == targetVehicleID {
		return nil, ErrFunctionReplaceSourceInvalid
	}
	v, err := s.loadVehicle(ctx, targetVehicleID)
	if err != nil {
		return nil, err
	}
	if d := s.sec.CanEditVehicleFunctions(actorID, v.OwnerUserID); !d.Allowed {
		return nil, ErrOnlyOwnerCanEdit
	}
	if _, err := s.loadVehicle(ctx, sourceVehicleID); err != nil {
		return nil, err
	}
	srcFns, err := s.ListForVehicle(ctx, sourceVehicleID)
	if err != nil {
		return nil, err
	}
	err = s.functions.Transaction(ctx, func(ctx context.Context) error {
		if err := s.functions.DeleteAllByVehicleID(ctx, v.ID); err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, f := range srcFns {
			vid := v.ID
			row := domain.DccFunction{
				VehicleID:  &vid,
				TemplateID: nil,
				Num:        f.Num,
				Name:       f.Name,
				Icon:       f.Icon,
				Position:   f.Position,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if err := s.functions.Insert(ctx, &row); err != nil {
				return err
			}
		}
		v.TemplateID = nil
		v.FunctionsDetachedAt = nil
		v.UpdatedAt = now
		return s.vehicles.Update(ctx, &v)
	})
	if err != nil {
		return nil, err
	}
	return s.ListForVehicle(ctx, targetVehicleID)
}

// UpsertTemplateSlot adds or updates one function on a template.
func (s *FunctionService) UpsertTemplateSlot(
	ctx context.Context,
	actorID uint,
	eff domain.EffectiveRoles,
	templateID uint,
	num uint8,
	in FunctionUpsertInput,
) (domain.DccFunction, error) {
	if !domain.ValidFunctionNum(num) {
		return domain.DccFunction{}, ErrFunctionNumInvalid
	}
	tpl, err := s.loadTemplate(ctx, templateID)
	if err != nil {
		return domain.DccFunction{}, err
	}
	if d := s.sec.CanEditTemplateFunctions(eff, actorID, tpl.OwnerUserID); !d.Allowed {
		return domain.DccFunction{}, ErrTemplateNotOwned
	}
	payload, err := validateFunctionInput(in)
	if err != nil {
		return domain.DccFunction{}, err
	}
	return s.upsertTemplateRow(ctx, templateID, num, payload)
}

// DeleteTemplateSlot removes one function from a template.
func (s *FunctionService) DeleteTemplateSlot(
	ctx context.Context,
	actorID uint,
	eff domain.EffectiveRoles,
	templateID uint,
	num uint8,
) error {
	if !domain.ValidFunctionNum(num) {
		return ErrFunctionNumInvalid
	}
	tpl, err := s.loadTemplate(ctx, templateID)
	if err != nil {
		return err
	}
	if d := s.sec.CanEditTemplateFunctions(eff, actorID, tpl.OwnerUserID); !d.Allowed {
		return ErrTemplateNotOwned
	}
	row, err := s.functions.FindByTemplateAndNum(ctx, templateID, num)
	if err != nil {
		if errors.Is(err, repo.ErrDccFunctionNotFound) {
			return ErrFunctionNotFound
		}
		return err
	}
	return s.functions.Delete(ctx, &row)
}

// ReorderTemplateSlots updates display order on a template.
func (s *FunctionService) ReorderTemplateSlots(
	ctx context.Context,
	actorID uint,
	eff domain.EffectiveRoles,
	templateID uint,
	positions []FunctionReorderEntry,
) error {
	tpl, err := s.loadTemplate(ctx, templateID)
	if err != nil {
		return err
	}
	if d := s.sec.CanEditTemplateFunctions(eff, actorID, tpl.OwnerUserID); !d.Allowed {
		return ErrTemplateNotOwned
	}
	return s.applyReorder(ctx, func(ctx context.Context, num uint8) (domain.DccFunction, error) {
		return s.functions.FindByTemplateAndNum(ctx, tpl.ID, num)
	}, positions)
}

func (s *FunctionService) loadVehicle(ctx context.Context, id uint) (domain.Vehicle, error) {
	row, err := s.vehicles.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			return domain.Vehicle{}, ErrVehicleNotFound
		}
		return domain.Vehicle{}, err
	}
	return row, nil
}

func (s *FunctionService) loadTemplate(ctx context.Context, id uint) (domain.VehicleTemplate, error) {
	row, err := s.templates.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleTemplateNotFound) {
			return domain.VehicleTemplate{}, ErrVehicleTemplateNotFound
		}
		return domain.VehicleTemplate{}, err
	}
	return row, nil
}

func (s *FunctionService) ensureDetached(ctx context.Context, v *domain.Vehicle) error {
	if v.TemplateID == nil || v.FunctionsDetachedAt != nil {
		return nil
	}
	tplFns, err := s.functions.ListByTemplateID(ctx, *v.TemplateID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, tf := range tplFns {
		vid := v.ID
		row := domain.DccFunction{
			VehicleID:  &vid,
			TemplateID: nil,
			Num:        tf.Num,
			Name:       tf.Name,
			Icon:       tf.Icon,
			Position:   tf.Position,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := s.functions.Insert(ctx, &row); err != nil {
			return err
		}
	}
	ts := now
	v.FunctionsDetachedAt = &ts
	v.UpdatedAt = now
	return s.vehicles.Update(ctx, v)
}

type validatedFunction struct {
	name     string
	icon     domain.FunctionIcon
	position int
}

func validateFunctionInput(in FunctionUpsertInput) (validatedFunction, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return validatedFunction{}, ErrFunctionNameRequired
	}
	if len(name) > maxFunctionNameLen {
		name = name[:maxFunctionNameLen]
	}
	if !in.Icon.IsValid() {
		return validatedFunction{}, ErrFunctionIconInvalid
	}
	return validatedFunction{
		name:     name,
		icon:     in.Icon,
		position: in.Position,
	}, nil
}

func (s *FunctionService) upsertVehicleRow(
	ctx context.Context,
	vehicleID uint,
	num uint8,
	in validatedFunction,
) (domain.DccFunction, error) {
	existing, findErr := s.functions.FindByVehicleAndNum(ctx, vehicleID, num)
	now := time.Now().UTC()
	if findErr == nil {
		existing.Name = in.name
		existing.Icon = in.icon
		existing.Position = in.position
		existing.UpdatedAt = now
		if err := s.functions.Update(ctx, &existing); err != nil {
			return domain.DccFunction{}, err
		}
		return existing, nil
	}
	if !errors.Is(findErr, repo.ErrDccFunctionNotFound) {
		return domain.DccFunction{}, findErr
	}
	vid := vehicleID
	row := domain.DccFunction{
		VehicleID:  &vid,
		TemplateID: nil,
		Num:        num,
		Name:       in.name,
		Icon:       in.icon,
		Position:   in.position,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.functions.Insert(ctx, &row); err != nil {
		if isUniqueViolation(err) {
			return domain.DccFunction{}, ErrFunctionNumTaken
		}
		return domain.DccFunction{}, err
	}
	return row, nil
}

func (s *FunctionService) upsertTemplateRow(
	ctx context.Context,
	templateID uint,
	num uint8,
	in validatedFunction,
) (domain.DccFunction, error) {
	existing, findErr := s.functions.FindByTemplateAndNum(ctx, templateID, num)
	now := time.Now().UTC()
	if findErr == nil {
		existing.Name = in.name
		existing.Icon = in.icon
		existing.Position = in.position
		existing.UpdatedAt = now
		if err := s.functions.Update(ctx, &existing); err != nil {
			return domain.DccFunction{}, err
		}
		return existing, nil
	}
	if !errors.Is(findErr, repo.ErrDccFunctionNotFound) {
		return domain.DccFunction{}, findErr
	}
	tid := templateID
	row := domain.DccFunction{
		VehicleID:  nil,
		TemplateID: &tid,
		Num:        num,
		Name:       in.name,
		Icon:       in.icon,
		Position:   in.position,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.functions.Insert(ctx, &row); err != nil {
		if isUniqueViolation(err) {
			return domain.DccFunction{}, ErrFunctionNumTaken
		}
		return domain.DccFunction{}, err
	}
	return row, nil
}

type findFn func(context.Context, uint8) (domain.DccFunction, error)

func (s *FunctionService) applyReorder(
	ctx context.Context,
	find findFn,
	positions []FunctionReorderEntry,
) error {
	now := time.Now().UTC()
	for _, p := range positions {
		if !domain.ValidFunctionNum(p.Num) {
			return ErrFunctionNumInvalid
		}
		row, err := find(ctx, p.Num)
		if err != nil {
			if errors.Is(err, repo.ErrDccFunctionNotFound) {
				return ErrFunctionNotFound
			}
			return err
		}
		row.Position = p.Position
		row.UpdatedAt = now
		if err := s.functions.Update(ctx, &row); err != nil {
			return err
		}
	}
	return nil
}

func toResolved(rows []domain.DccFunction, source string) []ResolvedFunction {
	out := make([]ResolvedFunction, 0, len(rows))
	for _, r := range rows {
		out = append(out, ResolvedFunction{
			Num:      r.Num,
			Name:     r.Name,
			Icon:     r.Icon,
			Position: r.Position,
			Source:   source,
		})
	}
	return out
}
