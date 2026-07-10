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

// DefaultAdminPIN is the well-known admin PIN seeded onto a freshly-created
// system layout (§7a.7). SHOULD be rotated after first login.
const DefaultAdminPIN = "0000"

// Layout implements layout CRUD + lifecycle rules (§3a.1, §4.1).
type Layout struct {
	layouts               *repo.Layouts
	interlockings         *repo.Interlockings
	layoutInterlockings   *repo.LayoutInterlockings
	commandStations       *repo.CommandStations
	layoutCommandStations *repo.LayoutCommandStations
	sec                   security.LayoutSecurityContext
}

// NewLayout constructs a service bound to a Layouts
// repository.
func NewLayout(
	layouts *repo.Layouts,
	interlockings *repo.Interlockings,
	layoutInterlockings *repo.LayoutInterlockings,
	commandStations *repo.CommandStations,
	layoutCommandStations *repo.LayoutCommandStations,
) *Layout {
	return &Layout{
		layouts:               layouts,
		interlockings:         interlockings,
		layoutInterlockings:   layoutInterlockings,
		commandStations:       commandStations,
		layoutCommandStations: layoutCommandStations,
	}
}

// EnsureSystemLayout inserts the bootstrap system layout row if it
// does not already exist. The operation is idempotent — calling it on
// every server startup is safe. The seeded row has
// Name = domain.SystemLayoutName, IsSystem = true, Locked = false,
// CreatedBy = 0 (no admin user owns it), and AdminPINHash = the
// argon2id digest of DefaultAdminPIN so the sudo flow
// has a comparable hash on day one.
//
// Returns true when the seed actually happened so the caller can
// emit a one-shot log line.
func (s *Layout) EnsureSystemLayout(ctx context.Context) (bool, error) {
	if _, err := s.layouts.FindSystem(ctx); err == nil {
		return false, nil
	} else if !errors.Is(err, repo.ErrLayoutNotFound) {
		return false, err
	}

	hash, err := helpers.HashPIN(DefaultAdminPIN)
	if err != nil {
		return false, err
	}

	now := time.Now().UTC()
	layout := domain.Layout{
		Name:         domain.SystemLayoutName,
		IsSystem:     true,
		Locked:       false,
		CreatedBy:    0,
		AdminPINHash: hash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.layouts.Insert(ctx, &layout); err != nil {
		return false, err
	}
	return true, nil
}

// ListAll returns every layout (admin view). The result is ordered
// with the system row first.
func (s *Layout) ListAll(ctx context.Context) ([]domain.Layout, error) {
	return s.layouts.ListAll(ctx)
}

// ListSelectable returns the rows the login dropdown should offer:
// every non-locked layout (§7a.1). When every layout is locked the
// system row is still returned so operators are not locked out of the
// workshop default.
func (s *Layout) ListSelectable(ctx context.Context) ([]domain.Layout, error) {
	rows, err := s.layouts.ListSelectable(ctx)
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}
	system, err := s.layouts.FindSystem(ctx)
	if err != nil {
		if errors.Is(err, repo.ErrLayoutNotFound) {
			return rows, nil
		}
		return nil, err
	}
	return []domain.Layout{system}, nil
}

// Get looks a layout up by id. Translates svcerrors.ErrLayoutNotFound from the
// repo into the service-level sentinel of the same name (different
// package).
func (s *Layout) Get(ctx context.Context, id uint) (domain.Layout, error) {
	layout, err := s.layouts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrLayoutNotFound) {
			return domain.Layout{}, svcerrors.ErrLayoutNotFound
		}
		return domain.Layout{}, err
	}
	return layout, nil
}

// GetSystem returns the bootstrap system layout. EnsureSystemLayout
// guarantees the row exists from the very first server boot, so an
// svcerrors.ErrLayoutNotFound here would be a serious database-level fault.
func (s *Layout) GetSystem(ctx context.Context) (domain.Layout, error) {
	layout, err := s.layouts.FindSystem(ctx)
	if err != nil {
		if errors.Is(err, repo.ErrLayoutNotFound) {
			return domain.Layout{}, svcerrors.ErrLayoutNotFound
		}
		return domain.Layout{}, err
	}
	return layout, nil
}

// LayoutCreateInput is the validated payload of Layout.Create.
type LayoutCreateInput struct {
	Name              string
	CreatedBy         uint
	InterlockingIDs   []uint
	CommandStationIDs []uint
	// AdminPIN is the layout's initial sudo PIN (§7a.7). Empty
	// means "seed with DefaultAdminPIN" — same UX as
	// the system layout, so a freshly-created layout already has
	// a usable PIN until the admin rotates it. Must satisfy
	// validateLayoutAdminPIN when non-empty.
	AdminPIN string
	// MaxVehiclesPerUser caps driven vehicles per user. Zero selects default.
	MaxVehiclesPerUser uint
}

// Create inserts a brand-new non-system layout. Name uniqueness and
// non-emptiness are enforced explicitly so the HTTP layer can return
// the matching 4xx code without parsing SQL error strings.
func (s *Layout) Create(ctx context.Context, eff domain.EffectiveRoles, in LayoutCreateInput) (domain.Layout, error) {
	if err := s.checkManageLayouts(eff); err != nil {
		return domain.Layout{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return domain.Layout{}, svcerrors.ErrLayoutNameRequired
	}
	if len(name) > validation.MaxLayoutNameLen {
		return domain.Layout{}, svcerrors.ErrLayoutNameRequired
	}
	// The literal Name `default` is reserved for the system row. A
	// non-system layout that picks the same name would either fail
	// the unique constraint or shadow the system row through the
	// `name` lookup — both are confusing. Reject early.
	if name == domain.SystemLayoutName {
		return domain.Layout{}, svcerrors.ErrLayoutNameTaken
	}

	if _, err := s.layouts.FindByName(ctx, name); err == nil {
		return domain.Layout{}, svcerrors.ErrLayoutNameTaken
	} else if !errors.Is(err, repo.ErrLayoutNotFound) {
		return domain.Layout{}, err
	}

	pin := in.AdminPIN
	if pin == "" {
		pin = DefaultAdminPIN
	}
	if err := validation.ValidateLayoutAdminPIN(pin); err != nil {
		return domain.Layout{}, err
	}
	maxVehicles, err := validation.SanitiseLayoutMaxVehiclesPerUser(in.MaxVehiclesPerUser)
	if err != nil {
		return domain.Layout{}, err
	}
	hash, err := helpers.HashPIN(pin)
	if err != nil {
		return domain.Layout{}, err
	}

	now := time.Now().UTC()
	layout := domain.Layout{
		Name:         name,
		IsSystem:     false,
		Locked:       false,
		CreatedBy:          in.CreatedBy,
		AdminPINHash:       hash,
		MaxVehiclesPerUser: maxVehicles,
		CreatedAt:          now,
		UpdatedAt:    now,
	}
	if err := s.layouts.Insert(ctx, &layout); err != nil {
		return domain.Layout{}, err
	}
	if err := s.setInterlockings(ctx, layout.ID, in.CreatedBy, in.InterlockingIDs); err != nil {
		return domain.Layout{}, err
	}
	if err := s.setCommandStations(ctx, layout.ID, in.CreatedBy, in.CommandStationIDs); err != nil {
		return domain.Layout{}, err
	}
	if err := s.validateMaxVehiclesAgainstLayoutCS(ctx, layout.ID, layout.EffectiveMaxVehiclesPerUser()); err != nil {
		return domain.Layout{}, err
	}
	return layout, nil
}

// UpdateMaxVehiclesPerUser sets the per-user driven-vehicle cap for a layout.
func (s *Layout) UpdateMaxVehiclesPerUser(ctx context.Context, eff domain.EffectiveRoles, id uint, max uint) (domain.Layout, error) {
	if err := s.checkManageLayouts(eff); err != nil {
		return domain.Layout{}, err
	}
	maxVehicles, err := validation.SanitiseLayoutMaxVehiclesPerUser(max)
	if err != nil {
		return domain.Layout{}, err
	}
	layout, err := s.Get(ctx, id)
	if err != nil {
		return domain.Layout{}, err
	}
	effective := maxVehicles
	if effective == 0 {
		effective = domain.DefaultLayoutMaxVehiclesPerUser
	}
	if err := s.validateMaxVehiclesAgainstLayoutCS(ctx, id, effective); err != nil {
		return domain.Layout{}, err
	}
	layout.MaxVehiclesPerUser = maxVehicles
	layout.UpdatedAt = time.Now().UTC()
	if err := s.layouts.Update(ctx, &layout); err != nil {
		return domain.Layout{}, err
	}
	return layout, nil
}

func (s *Layout) validateMaxVehiclesAgainstLayoutCS(ctx context.Context, layoutID uint, maxVehicles uint) error {
	csIDs, err := s.layoutCommandStations.CommandStationIDsForLayout(ctx, layoutID)
	if err != nil {
		return err
	}
	if len(csIDs) == 0 {
		return nil
	}
	minSlots := uint(0)
	for _, csID := range csIDs {
		cs, err := s.commandStations.FindByID(ctx, csID)
		if err != nil {
			return err
		}
		if !cs.Kind.IsLocoNet() {
			continue
		}
		slots := cs.EffectiveMaxLoconetSlots()
		if minSlots == 0 || slots < minSlots {
			minSlots = slots
		}
	}
	if minSlots > 0 && maxVehicles > minSlots {
		return svcerrors.ErrLayoutMaxVehiclesExceedsSlotBudget
	}
	return nil
}

// UpdateAdminPIN rotates the layout's admin PIN.
//
// The HTTP layer guards the call with the policy that "only a
// non-sudo permanent admin may rotate the layout admin PIN" — see
// §7a.3 / §7a.7.
func (s *Layout) UpdateAdminPIN(ctx context.Context, eff domain.EffectiveRoles, id uint, newPIN string) (domain.Layout, error) {
	if err := s.checkManageLayouts(eff); err != nil {
		return domain.Layout{}, err
	}
	layout, err := s.Get(ctx, id)
	if err != nil {
		return domain.Layout{}, err
	}
	if newPIN == "" {
		return layout, nil
	}
	if err := validation.ValidateLayoutAdminPIN(newPIN); err != nil {
		return domain.Layout{}, err
	}
	hash, err := helpers.HashPIN(newPIN)
	if err != nil {
		return domain.Layout{}, err
	}
	layout.AdminPINHash = hash
	layout.UpdatedAt = time.Now().UTC()
	if err := s.layouts.Update(ctx, &layout); err != nil {
		return domain.Layout{}, err
	}
	return layout, nil
}

// VerifyAdminPIN compares a candidate PIN against the layout's
// stored digest (§7a.7). Returns:
//
//   - nil — match, caller may proceed with the elevation;
//   - svcerrors.ErrLayoutAdminPINUnset — the layout has no PIN yet (an
//     empty digest never matches anything);
//   - svcerrors.ErrLayoutAdminPINMismatch — wrong PIN (the rate-limiter must
//     count this towards the brute-force counter);
//   - svcerrors.ErrLayoutNotFound — layout id is unknown.
func (s *Layout) VerifyAdminPIN(ctx context.Context, id uint, pin string) error {
	layout, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if layout.AdminPINHash == "" {
		return svcerrors.ErrLayoutAdminPINUnset
	}
	if err := helpers.VerifyPIN(pin, layout.AdminPINHash); err != nil {
		return svcerrors.ErrLayoutAdminPINMismatch
	}
	return nil
}

// Rename updates the layout's Name. The system row rejects with
// svcerrors.ErrSystemLayoutImmutable so the UI can keep its row read-only.
func (s *Layout) Rename(ctx context.Context, eff domain.EffectiveRoles, id uint, newName string) (domain.Layout, error) {
	if err := s.checkManageLayouts(eff); err != nil {
		return domain.Layout{}, err
	}
	layout, err := s.Get(ctx, id)
	if err != nil {
		return domain.Layout{}, err
	}
	if layout.IsSystem {
		return domain.Layout{}, svcerrors.ErrSystemLayoutImmutable
	}
	name := strings.TrimSpace(newName)
	if name == "" {
		return domain.Layout{}, svcerrors.ErrLayoutNameRequired
	}
	if len(name) > validation.MaxLayoutNameLen {
		return domain.Layout{}, svcerrors.ErrLayoutNameRequired
	}
	if name == domain.SystemLayoutName {
		return domain.Layout{}, svcerrors.ErrLayoutNameTaken
	}
	if name == layout.Name {
		return layout, nil
	}

	if other, err := s.layouts.FindByName(ctx, name); err == nil {
		if other.ID != layout.ID {
			return domain.Layout{}, svcerrors.ErrLayoutNameTaken
		}
	} else if !errors.Is(err, repo.ErrLayoutNotFound) {
		return domain.Layout{}, err
	}

	layout.Name = name
	layout.UpdatedAt = time.Now().UTC()
	if err := s.layouts.Update(ctx, &layout); err != nil {
		return domain.Layout{}, err
	}
	return layout, nil
}

// Delete removes the layout. The system row rejects with
// svcerrors.ErrSystemLayoutUndeletable (§4.1 DELETE).
//
// Note: §4.1 also says deletion must 409 when any live drive
// session is still pinned to the layout. Drive sessions are part of
// a later milestone (M4 command-station bring-up), so the check
// will be added together with `DriveSession`. The system row guard
// covers the most-important footgun for now.
func (s *Layout) Delete(ctx context.Context, eff domain.EffectiveRoles, id uint) error {
	if err := s.checkManageLayouts(eff); err != nil {
		return err
	}
	layout, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if layout.IsSystem {
		return svcerrors.ErrSystemLayoutUndeletable
	}
	return s.layouts.Delete(ctx, &layout)
}

// SetLocked toggles the Locked flag. Idempotent on either branch.
// Returns the updated row so the caller can echo the new state in
// the HTTP response.
func (s *Layout) SetLocked(ctx context.Context, id uint, locked bool) (domain.Layout, error) {
	layout, err := s.Get(ctx, id)
	if err != nil {
		return domain.Layout{}, err
	}
	if layout.Locked == locked {
		return layout, nil
	}
	layout.Locked = locked
	layout.UpdatedAt = time.Now().UTC()
	if err := s.layouts.Update(ctx, &layout); err != nil {
		return domain.Layout{}, err
	}
	return layout, nil
}

// ListInterlockings returns interlockings whitelisted for a layout.
func (s *Layout) ListInterlockings(ctx context.Context, layoutID uint) ([]domain.Interlocking, error) {
	if _, err := s.Get(ctx, layoutID); err != nil {
		return nil, err
	}
	ids, err := s.layoutInterlockings.InterlockingIDsForLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	return s.interlockings.ListByIDs(ctx, ids)
}

// LayoutIDsForCommandStation returns every layout the command station
// is attached to via layout_command_stations. The system layout is
// omitted because its attachment set is virtual (§4.1).
func (s *Layout) LayoutIDsForCommandStation(ctx context.Context, commandStationID uint) ([]uint, error) {
	rows, err := s.layoutCommandStations.ListByCommandStation(ctx, commandStationID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.LayoutID)
	}
	return ids, nil
}

// CommandStationIDsForLayout returns the ids of command stations
// available on a layout. For the system layout this is every row in
// the catalogue (virtual attachment set, §4.1).
func (s *Layout) CommandStationIDsForLayout(ctx context.Context, layoutID uint) ([]uint, error) {
	layout, err := s.Get(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	if layout.IsSystem {
		rows, err := s.commandStations.ListAll(ctx)
		if err != nil {
			return nil, err
		}
		ids := make([]uint, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		return ids, nil
	}
	return s.layoutCommandStations.CommandStationIDsForLayout(ctx, layoutID)
}

// ListCommandStations returns command stations attached to a layout.
// For the system layout the live catalogue is synthesised (§4.1).
func (s *Layout) ListCommandStations(ctx context.Context, layoutID uint) ([]domain.CommandStation, error) {
	layout, err := s.Get(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	if layout.IsSystem {
		return s.commandStations.ListAll(ctx)
	}
	ids, err := s.layoutCommandStations.CommandStationIDsForLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	return s.commandStations.ListByIDs(ctx, ids)
}

// SetCommandStations replaces the entire command-station attachment
// set for a non-system layout. Unknown ids reject with
// svcerrors.ErrCommandStationNotFound. The system layout rejects with
// svcerrors.ErrSystemLayoutCommandStationsImmutable.
func (s *Layout) SetCommandStations(ctx context.Context, eff domain.EffectiveRoles, layoutID, addedBy uint, commandStationIDs []uint) ([]domain.CommandStation, error) {
	if err := s.checkManageLayouts(eff); err != nil {
		return nil, err
	}
	layout, err := s.Get(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	if layout.IsSystem {
		return nil, svcerrors.ErrSystemLayoutCommandStationsImmutable
	}
	if err := s.setCommandStations(ctx, layoutID, addedBy, commandStationIDs); err != nil {
		return nil, err
	}
	return s.ListCommandStations(ctx, layoutID)
}

// SetInterlockings replaces the entire interlocking whitelist for a
// layout with the supplied id set. Unknown ids reject with
// svcerrors.ErrInterlockingNotFound. Duplicate ids in the input are ignored.
func (s *Layout) SetInterlockings(ctx context.Context, eff domain.EffectiveRoles, layoutID, addedBy uint, interlockingIDs []uint) ([]domain.Interlocking, error) {
	if err := s.checkManageLayouts(eff); err != nil {
		return nil, err
	}
	if _, err := s.Get(ctx, layoutID); err != nil {
		return nil, err
	}
	if err := s.setInterlockings(ctx, layoutID, addedBy, interlockingIDs); err != nil {
		return nil, err
	}
	return s.ListInterlockings(ctx, layoutID)
}

func (s *Layout) checkManageLayouts(eff domain.EffectiveRoles) error {
	decision := s.sec.CanManageLayouts(eff)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case security.ReasonForbidden:
		return svcerrors.ErrLayoutForbidden
	default:
		return errors.New(decision.Reason)
	}
}

func (s *Layout) setCommandStations(ctx context.Context, layoutID, addedBy uint, commandStationIDs []uint) error {
	seen := make(map[uint]struct{}, len(commandStationIDs))
	unique := make([]uint, 0, len(commandStationIDs))
	for _, id := range commandStationIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if _, err := s.commandStations.FindByID(ctx, id); err != nil {
			if errors.Is(err, repo.ErrCommandStationNotFound) {
				return svcerrors.ErrCommandStationNotFound
			}
			return err
		}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return svcerrors.ErrLayoutNeedsAtLeastOneCommandStation
	}

	if err := s.layoutCommandStations.DeleteAllForLayout(ctx, layoutID); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, id := range unique {
		row := domain.LayoutCommandStation{
			LayoutID:         layoutID,
			CommandStationID: id,
			AddedByUserID:    addedBy,
			AddedAt:          now,
		}
		if err := s.layoutCommandStations.Attach(ctx, &row); err != nil {
			return err
		}
	}
	return nil
}

func (s *Layout) setInterlockings(ctx context.Context, layoutID, addedBy uint, interlockingIDs []uint) error {
	seen := make(map[uint]struct{}, len(interlockingIDs))
	unique := make([]uint, 0, len(interlockingIDs))
	for _, id := range interlockingIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if _, err := s.interlockings.FindByID(ctx, id); err != nil {
			if errors.Is(err, repo.ErrInterlockingNotFound) {
				return svcerrors.ErrInterlockingNotFound
			}
			return err
		}
		unique = append(unique, id)
	}

	if err := s.layoutInterlockings.DeleteAllForLayout(ctx, layoutID); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, id := range unique {
		row := domain.LayoutInterlocking{
			LayoutID:       layoutID,
			InterlockingID: id,
			AddedByUserID:  addedBy,
			AddedAt:        now,
		}
		if err := s.layoutInterlockings.Insert(ctx, &row); err != nil {
			return err
		}
	}
	return nil
}

// ValidateForLogin loads a layout by id and rejects locked rows with
// svcerrors.ErrLayoutLocked. The system layout is accepted while locked
// only when no other layout is selectable — the same fallback as
// ListSelectable. Used by AuthService.Login to fulfil §7a.1 steps 2 + 3
// without coupling AuthService to the Layouts repo directly.
func (s *Layout) ValidateForLogin(ctx context.Context, id uint) (domain.Layout, error) {
	layout, err := s.Get(ctx, id)
	if err != nil {
		return domain.Layout{}, err
	}
	if !layout.Locked {
		return layout, nil
	}
	if layout.IsSystem && s.noSelectableLayouts(ctx) {
		return layout, nil
	}
	return domain.Layout{}, svcerrors.ErrLayoutLocked
}

// noSelectableLayouts reports whether the login dropdown would be
// empty without the system-layout fallback.
func (s *Layout) noSelectableLayouts(ctx context.Context) bool {
	rows, err := s.layouts.ListSelectable(ctx)
	return err == nil && len(rows) == 0
}
