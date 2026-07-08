package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/helpers"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

// ResolvedFunction is the effective slot a throttle or editor displays.
type ResolvedFunction struct {
	Num                 uint8
	Name                string
	Icon                domain.FunctionIcon
	Position            int
	Momentary           bool
	MomentaryDurationMs int
	Source              string // "template" | "vehicle"
}

// FunctionUpsertInput is the validated payload for upserting one slot.
type FunctionUpsertInput = validation.FunctionUpsertInput

// FunctionReorderEntry maps a function number to its display position.
type FunctionReorderEntry struct {
	Num      uint8
	Position int
}

// VehicleFunctionCatalogueEntry is one vehicle with its resolved function list.
type VehicleFunctionCatalogueEntry struct {
	VehicleID         domain.VehicleID
	VehicleName       string
	OwnerID           uint
	OwnerLogin        string
	OwnerOrganization string
	DCCAddress        *uint16
	Kind              domain.VehicleKind
	Functions         []ResolvedFunction
}

// Function manages dcc_functions for vehicles and templates (§3a.6).
type Function struct {
	functions *repo.DccFunctions
	vehicles  *repo.Vehicles
	templates *repo.VehicleTemplates
	users     *repo.Users
	sec       security.FunctionSecurityContext
}

// NewFunction constructs a Function use-case handler.
func NewFunction(
	f *repo.DccFunctions,
	v *repo.Vehicles,
	t *repo.VehicleTemplates,
	u *repo.Users,
) *Function {
	return &Function{functions: f, vehicles: v, templates: t, users: u}
}

// ListIcons returns the closed icon catalogue.
func (f *Function) ListIcons() []domain.FunctionIcon {
	return domain.FunctionIcons()
}

// ListForVehicle returns the resolved function list for a vehicle.
func (f *Function) ListForVehicle(ctx context.Context, vehicleID domain.VehicleID) ([]ResolvedFunction, error) {
	v, err := f.loadVehicle(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	if v.TemplateID != nil && v.FunctionsDetachedAt == nil {
		rows, err := f.functions.ListByTemplateID(ctx, *v.TemplateID)
		if err != nil {
			return nil, err
		}
		return toResolved(rows, "template"), nil
	}
	rows, err := f.functions.ListByVehicleID(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	return toResolved(rows, "vehicle"), nil
}

// ListForVehicles resolves function catalogues for many vehicles in batch.
// Template-inheriting vehicles share one ListByTemplateID per distinct template;
// detached vehicles are loaded with a single ListByVehicleIDs query.
func (f *Function) ListForVehicles(ctx context.Context, vehicles []domain.Vehicle) (map[domain.VehicleID][]ResolvedFunction, error) {
	out := make(map[domain.VehicleID][]ResolvedFunction, len(vehicles))
	if len(vehicles) == 0 {
		return out, nil
	}
	var detachedIDs []domain.VehicleID
	templateToVehicles := make(map[uint][]domain.VehicleID)
	for _, v := range vehicles {
		if v.TemplateID != nil && v.FunctionsDetachedAt == nil {
			tid := *v.TemplateID
			templateToVehicles[tid] = append(templateToVehicles[tid], v.ID)
		} else {
			detachedIDs = append(detachedIDs, v.ID)
		}
	}
	if len(detachedIDs) > 0 {
		rows, err := f.functions.ListByVehicleIDs(ctx, detachedIDs)
		if err != nil {
			return nil, err
		}
		byVehicle := groupFunctionsByVehicleID(rows)
		for _, id := range detachedIDs {
			out[id] = toResolved(byVehicle[id], "vehicle")
		}
	}
	for tid, ids := range templateToVehicles {
		rows, err := f.functions.ListByTemplateID(ctx, tid)
		if err != nil {
			return nil, err
		}
		resolved := toResolved(rows, "template")
		for _, id := range ids {
			out[id] = resolved
		}
	}
	return out, nil
}

// ListFunctionCatalogue returns every vehicle with at least one function.
func (f *Function) ListFunctionCatalogue(ctx context.Context) ([]VehicleFunctionCatalogueEntry, error) {
	vehicles, err := f.vehicles.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	logins := make(map[uint]struct {
		login        string
		organization string
	})
	out := make([]VehicleFunctionCatalogueEntry, 0, len(vehicles))
	for _, v := range vehicles {
		fns, err := f.ListForVehicle(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		if len(fns) == 0 {
			continue
		}
		info, ok := logins[v.OwnerUserID]
		if !ok {
			u, err := f.users.FindByID(ctx, v.OwnerUserID)
			if err != nil {
				info.login = "?"
			} else {
				info.login = u.Login
				info.organization = u.Organization
			}
			logins[v.OwnerUserID] = info
		}
		out = append(out, VehicleFunctionCatalogueEntry{
			VehicleID:         v.ID,
			VehicleName:       v.Name,
			OwnerID:           v.OwnerUserID,
			OwnerLogin:        info.login,
			OwnerOrganization: info.organization,
			DCCAddress:        v.DCCAddress,
			Kind:              v.Kind,
			Functions:         fns,
		})
	}
	return out, nil
}

// ListForTemplate returns function rows owned by a template.
func (f *Function) ListForTemplate(ctx context.Context, templateID uint) ([]ResolvedFunction, error) {
	if _, err := f.loadTemplate(ctx, templateID); err != nil {
		return nil, err
	}
	rows, err := f.functions.ListByTemplateID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	return toResolved(rows, "template"), nil
}

// UpsertVehicleSlot adds or updates one function on a vehicle (owner only).
func (f *Function) UpsertVehicleSlot(
	ctx context.Context,
	actorID uint,
	vehicleID domain.VehicleID,
	num uint8,
	in FunctionUpsertInput,
) (domain.DccFunction, error) {
	if !domain.ValidFunctionNum(num) {
		return domain.DccFunction{}, svcerrors.ErrFunctionNumInvalid
	}
	v, err := f.loadVehicle(ctx, vehicleID)
	if err != nil {
		return domain.DccFunction{}, err
	}
	if d := f.sec.CanEditVehicleFunctions(actorID, v.OwnerUserID); !d.Allowed {
		return domain.DccFunction{}, svcerrors.ErrOnlyOwnerCanEdit
	}
	payload, err := validation.ValidateFunctionUpsert(in)
	if err != nil {
		return domain.DccFunction{}, err
	}

	var out domain.DccFunction
	err = f.functions.Transaction(ctx, func(ctx context.Context) error {
		if err := f.ensureDetached(ctx, &v); err != nil {
			return err
		}
		row, err := f.upsertVehicleRow(ctx, v.ID, num, payload)
		if err != nil {
			return err
		}
		out = row
		return nil
	})
	return out, err
}

// DeleteVehicleSlot removes one function from a vehicle (owner only).
func (f *Function) DeleteVehicleSlot(
	ctx context.Context,
	actorID uint, vehicleID domain.VehicleID,
	num uint8,
) error {
	if !domain.ValidFunctionNum(num) {
		return svcerrors.ErrFunctionNumInvalid
	}
	v, err := f.loadVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	if d := f.sec.CanEditVehicleFunctions(actorID, v.OwnerUserID); !d.Allowed {
		return svcerrors.ErrOnlyOwnerCanEdit
	}
	return f.functions.Transaction(ctx, func(ctx context.Context) error {
		if err := f.ensureDetached(ctx, &v); err != nil {
			return err
		}
		row, err := f.functions.FindByVehicleAndNum(ctx, v.ID, num)
		if err != nil {
			if errors.Is(err, repo.ErrDccFunctionNotFound) {
				return svcerrors.ErrFunctionNotFound
			}
			return err
		}
		return f.functions.Delete(ctx, &row)
	})
}

// ReorderVehicleSlots updates display order on a vehicle (owner only).
func (f *Function) ReorderVehicleSlots(
	ctx context.Context,
	actorID uint, vehicleID domain.VehicleID,
	positions []FunctionReorderEntry,
) error {
	v, err := f.loadVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	if d := f.sec.CanEditVehicleFunctions(actorID, v.OwnerUserID); !d.Allowed {
		return svcerrors.ErrOnlyOwnerCanEdit
	}
	return f.functions.Transaction(ctx, func(ctx context.Context) error {
		if err := f.ensureDetached(ctx, &v); err != nil {
			return err
		}
		return f.applyReorder(ctx, func(ctx context.Context, num uint8) (domain.DccFunction, error) {
			return f.functions.FindByVehicleAndNum(ctx, v.ID, num)
		}, positions)
	})
}

// AttachVehicleToTemplate links a vehicle to a template function list.
func (f *Function) AttachVehicleToTemplate(
	ctx context.Context,
	actorID uint, vehicleID domain.VehicleID, templateID uint,
) ([]ResolvedFunction, error) {
	if templateID == 0 {
		return nil, svcerrors.ErrVehicleTemplateNotFound
	}
	v, err := f.loadVehicle(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	if d := f.sec.CanEditVehicleFunctions(actorID, v.OwnerUserID); !d.Allowed {
		return nil, svcerrors.ErrOnlyOwnerCanEdit
	}
	if _, err := f.loadTemplate(ctx, templateID); err != nil {
		return nil, err
	}
	err = f.functions.Transaction(ctx, func(ctx context.Context) error {
		if err := f.functions.DeleteAllByVehicleID(ctx, v.ID); err != nil {
			return err
		}
		tid := templateID
		v.TemplateID = &tid
		v.FunctionsDetachedAt = nil
		v.UpdatedAt = time.Now().UTC()
		return f.vehicles.Update(ctx, &v)
	})
	if err != nil {
		return nil, err
	}
	return f.ListForVehicle(ctx, vehicleID)
}

// CopyVehicleFunctionsFromVehicle replaces the target list from a source snapshot.
func (f *Function) CopyVehicleFunctionsFromVehicle(
	ctx context.Context,
	actorID uint, targetVehicleID, sourceVehicleID domain.VehicleID,
) ([]ResolvedFunction, error) {
	if sourceVehicleID.IsZero() || sourceVehicleID == targetVehicleID {
		return nil, svcerrors.ErrFunctionReplaceSourceInvalid
	}
	v, err := f.loadVehicle(ctx, targetVehicleID)
	if err != nil {
		return nil, err
	}
	if d := f.sec.CanEditVehicleFunctions(actorID, v.OwnerUserID); !d.Allowed {
		return nil, svcerrors.ErrOnlyOwnerCanEdit
	}
	if _, err := f.loadVehicle(ctx, sourceVehicleID); err != nil {
		return nil, err
	}
	srcFns, err := f.ListForVehicle(ctx, sourceVehicleID)
	if err != nil {
		return nil, err
	}
	err = f.functions.Transaction(ctx, func(ctx context.Context) error {
		if err := f.functions.DeleteAllByVehicleID(ctx, v.ID); err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, fn := range srcFns {
			vid := v.ID
			row := domain.DccFunction{
				VehicleID:  &vid,
				TemplateID: nil,
				Num:        fn.Num,
				Name:       fn.Name,
				Icon:       fn.Icon,
				Position:   fn.Position,
				Momentary:           fn.Momentary,
				MomentaryDurationMs: fn.MomentaryDurationMs,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if err := f.functions.Insert(ctx, &row); err != nil {
				return err
			}
		}
		v.TemplateID = nil
		v.FunctionsDetachedAt = nil
		v.UpdatedAt = now
		return f.vehicles.Update(ctx, &v)
	})
	if err != nil {
		return nil, err
	}
	return f.ListForVehicle(ctx, targetVehicleID)
}

// UpsertTemplateSlot adds or updates one function on a template.
func (f *Function) UpsertTemplateSlot(
	ctx context.Context,
	actorID uint,
	eff domain.EffectiveRoles,
	templateID uint,
	num uint8,
	in FunctionUpsertInput,
) (domain.DccFunction, error) {
	if !domain.ValidFunctionNum(num) {
		return domain.DccFunction{}, svcerrors.ErrFunctionNumInvalid
	}
	tpl, err := f.loadTemplate(ctx, templateID)
	if err != nil {
		return domain.DccFunction{}, err
	}
	if d := f.sec.CanEditTemplateFunctions(eff, actorID, tpl.OwnerUserID); !d.Allowed {
		return domain.DccFunction{}, svcerrors.ErrTemplateNotOwned
	}
	payload, err := validation.ValidateFunctionUpsert(in)
	if err != nil {
		return domain.DccFunction{}, err
	}
	return f.upsertTemplateRow(ctx, templateID, num, payload)
}

// DeleteTemplateSlot removes one function from a template.
func (f *Function) DeleteTemplateSlot(
	ctx context.Context,
	actorID uint,
	eff domain.EffectiveRoles,
	templateID uint,
	num uint8,
) error {
	if !domain.ValidFunctionNum(num) {
		return svcerrors.ErrFunctionNumInvalid
	}
	tpl, err := f.loadTemplate(ctx, templateID)
	if err != nil {
		return err
	}
	if d := f.sec.CanEditTemplateFunctions(eff, actorID, tpl.OwnerUserID); !d.Allowed {
		return svcerrors.ErrTemplateNotOwned
	}
	row, err := f.functions.FindByTemplateAndNum(ctx, templateID, num)
	if err != nil {
		if errors.Is(err, repo.ErrDccFunctionNotFound) {
			return svcerrors.ErrFunctionNotFound
		}
		return err
	}
	return f.functions.Delete(ctx, &row)
}

// ReorderTemplateSlots updates display order on a template.
func (f *Function) ReorderTemplateSlots(
	ctx context.Context,
	actorID uint,
	eff domain.EffectiveRoles,
	templateID uint,
	positions []FunctionReorderEntry,
) error {
	tpl, err := f.loadTemplate(ctx, templateID)
	if err != nil {
		return err
	}
	if d := f.sec.CanEditTemplateFunctions(eff, actorID, tpl.OwnerUserID); !d.Allowed {
		return svcerrors.ErrTemplateNotOwned
	}
	return f.applyReorder(ctx, func(ctx context.Context, num uint8) (domain.DccFunction, error) {
		return f.functions.FindByTemplateAndNum(ctx, tpl.ID, num)
	}, positions)
}

func (f *Function) loadVehicle(ctx context.Context, id domain.VehicleID) (domain.Vehicle, error) {
	row, err := f.vehicles.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			return domain.Vehicle{}, svcerrors.ErrVehicleNotFound
		}
		return domain.Vehicle{}, err
	}
	return row, nil
}

func (f *Function) loadTemplate(ctx context.Context, id uint) (domain.VehicleTemplate, error) {
	row, err := f.templates.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleTemplateNotFound) {
			return domain.VehicleTemplate{}, svcerrors.ErrVehicleTemplateNotFound
		}
		return domain.VehicleTemplate{}, err
	}
	return row, nil
}

func (f *Function) ensureDetached(ctx context.Context, v *domain.Vehicle) error {
	if v.TemplateID == nil || v.FunctionsDetachedAt != nil {
		return nil
	}
	tplFns, err := f.functions.ListByTemplateID(ctx, *v.TemplateID)
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
			Momentary:           tf.Momentary,
			MomentaryDurationMs: tf.MomentaryDurationMs,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := f.functions.Insert(ctx, &row); err != nil {
			return err
		}
	}
	ts := now
	v.FunctionsDetachedAt = &ts
	v.UpdatedAt = now
	return f.vehicles.Update(ctx, v)
}

func (f *Function) upsertVehicleRow(
	ctx context.Context,
	vehicleID domain.VehicleID,
	num uint8,
	in validation.ValidatedFunction,
) (domain.DccFunction, error) {
	existing, findErr := f.functions.FindByVehicleAndNum(ctx, vehicleID, num)
	now := time.Now().UTC()
	if findErr == nil {
		existing.Name = in.Name
		existing.Icon = in.Icon
		existing.Position = in.Position
		existing.Momentary = in.Momentary
		existing.MomentaryDurationMs = in.MomentaryDurationMs
		existing.UpdatedAt = now
		if err := f.functions.Update(ctx, &existing); err != nil {
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
		Name:       in.Name,
		Icon:       in.Icon,
		Position:   in.Position,
		Momentary:           in.Momentary,
		MomentaryDurationMs: in.MomentaryDurationMs,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := f.functions.Insert(ctx, &row); err != nil {
		if helpers.IsUniqueViolation(err) {
			return domain.DccFunction{}, svcerrors.ErrFunctionNumTaken
		}
		return domain.DccFunction{}, err
	}
	return row, nil
}

func (f *Function) upsertTemplateRow(
	ctx context.Context,
	templateID uint,
	num uint8,
	in validation.ValidatedFunction,
) (domain.DccFunction, error) {
	existing, findErr := f.functions.FindByTemplateAndNum(ctx, templateID, num)
	now := time.Now().UTC()
	if findErr == nil {
		existing.Name = in.Name
		existing.Icon = in.Icon
		existing.Position = in.Position
		existing.Momentary = in.Momentary
		existing.MomentaryDurationMs = in.MomentaryDurationMs
		existing.UpdatedAt = now
		if err := f.functions.Update(ctx, &existing); err != nil {
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
		Name:       in.Name,
		Icon:       in.Icon,
		Position:   in.Position,
		Momentary:           in.Momentary,
		MomentaryDurationMs: in.MomentaryDurationMs,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := f.functions.Insert(ctx, &row); err != nil {
		if helpers.IsUniqueViolation(err) {
			return domain.DccFunction{}, svcerrors.ErrFunctionNumTaken
		}
		return domain.DccFunction{}, err
	}
	return row, nil
}

type functionFindFn func(context.Context, uint8) (domain.DccFunction, error)

func (f *Function) applyReorder(
	ctx context.Context,
	find functionFindFn,
	positions []FunctionReorderEntry,
) error {
	now := time.Now().UTC()
	for _, p := range positions {
		if !domain.ValidFunctionNum(p.Num) {
			return svcerrors.ErrFunctionNumInvalid
		}
		row, err := find(ctx, p.Num)
		if err != nil {
			if errors.Is(err, repo.ErrDccFunctionNotFound) {
				return svcerrors.ErrFunctionNotFound
			}
			return err
		}
		row.Position = p.Position
		row.UpdatedAt = now
		if err := f.functions.Update(ctx, &row); err != nil {
			return err
		}
	}
	return nil
}

func toResolved(rows []domain.DccFunction, source string) []ResolvedFunction {
	out := make([]ResolvedFunction, 0, len(rows))
	for _, r := range rows {
		out = append(out, ResolvedFunction{
			Num:                 r.Num,
			Name:                r.Name,
			Icon:                r.Icon,
			Position:            r.Position,
			Momentary:           r.Momentary,
			MomentaryDurationMs: r.MomentaryDurationMs,
			Source:              source,
		})
	}
	return out
}

func groupFunctionsByVehicleID(rows []domain.DccFunction) map[domain.VehicleID][]domain.DccFunction {
	m := make(map[domain.VehicleID][]domain.DccFunction)
	for _, r := range rows {
		if r.VehicleID == nil {
			continue
		}
		m[*r.VehicleID] = append(m[*r.VehicleID], r)
	}
	return m
}
