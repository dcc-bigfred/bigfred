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
	ErrFunctionKindInvalid     = errors.New("function_kind_invalid")
	ErrFunctionNameRequired    = errors.New("function_name_required")
	ErrFunctionNumTaken        = errors.New("function_num_taken")
	ErrFunctionNotFound        = errors.New("function_not_found")
	ErrOnlyOwnerCanEdit        = errors.New("only_owner_can_edit")
	ErrTemplateNotOwned        = errors.New("template_not_owned")
)

const maxFunctionNameLen = 64

// ResolvedFunction is the effective slot a throttle or editor displays.
type ResolvedFunction struct {
	Num      uint8
	Name     string
	Icon     domain.FunctionIcon
	Kind     domain.FunctionKind
	Position int
	Source   string // "template" | "vehicle"
}

// FunctionUpsertInput is the validated payload for upserting one slot.
type FunctionUpsertInput struct {
	Name     string
	Icon     domain.FunctionIcon
	Kind     domain.FunctionKind
	Position int
}

// FunctionReorderEntry maps a function number to its display position.
type FunctionReorderEntry struct {
	Num      uint8
	Position int
}

// FunctionService manages dcc_functions for vehicles and templates.
type FunctionService struct {
	functions *repo.DccFunctions
	vehicles  *repo.Vehicles
	templates *repo.VehicleTemplates
	sec       security.FunctionSecurityContext
}

// NewFunctionService constructs a FunctionService.
func NewFunctionService(
	f *repo.DccFunctions,
	v *repo.Vehicles,
	t *repo.VehicleTemplates,
) *FunctionService {
	return &FunctionService{functions: f, vehicles: v, templates: t}
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
			Kind:       tf.Kind,
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
	kind     domain.FunctionKind
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
	if !in.Kind.IsValid() {
		return validatedFunction{}, ErrFunctionKindInvalid
	}
	return validatedFunction{
		name:     name,
		icon:     in.Icon,
		kind:     in.Kind,
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
		existing.Kind = in.kind
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
		Kind:       in.kind,
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
		existing.Kind = in.kind
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
		Kind:       in.kind,
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
			Kind:     r.Kind,
			Position: r.Position,
			Source:   source,
		})
	}
	return out
}
