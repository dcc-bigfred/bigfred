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

// Layout-related sentinel errors. They are machine-readable on
// purpose so the HTTP handler can map each to a status code + an
// `errors` namespace key on the frontend. The names mirror the
// `error` strings called out in §4.1 / §3a.1.
var (
	// ErrLayoutNotFound is returned by Get / Update / Delete /
	// SetLocked when no row matches the requested id. The HTTP layer
	// turns it into 404 / 422 depending on the caller's intent.
	ErrLayoutNotFound = errors.New("layout_not_found")

	// ErrLayoutLocked is returned by AuthService.Login when the
	// caller picks a locked row out of band (the dropdown never
	// offers locked rows, but a hand-crafted request can still hit
	// the endpoint per §7a.1 step 3).
	ErrLayoutLocked = errors.New("layout_locked")

	// ErrLayoutNameTaken is the unique-name violation surfaced as a
	// machine code so the UI can highlight the right field.
	ErrLayoutNameTaken = errors.New("layout_name_taken")

	// ErrLayoutNameRequired is returned for blank/whitespace-only
	// names.
	ErrLayoutNameRequired = errors.New("layout_name_required")

	// ErrSystemLayoutImmutable is returned for any attempted rename
	// of the IsSystem row (§4.1 PUT /api/v1/layouts/{id}).
	ErrSystemLayoutImmutable = errors.New("default_layout_immutable")

	// ErrSystemLayoutUndeletable is returned for DELETE on the
	// system row.
	ErrSystemLayoutUndeletable = errors.New("default_layout_undeletable")

	// ErrLayoutAdminPINInvalid is returned when the supplied PIN
	// does not satisfy the digit / length policy (§7a.7). The same
	// code is used for "PIN supplied at create time but malformed"
	// and "PIN being rotated is malformed".
	ErrLayoutAdminPINInvalid = errors.New("layout_admin_pin_invalid")

	// ErrLayoutAdminPINUnset is returned by VerifyAdminPIN when the
	// layout's `admin_pin_hash` column is empty (the post-migration
	// default before any admin has rotated it). The HTTP layer maps
	// it to 422 so the UI can prompt the admin to set the PIN
	// before any sudo elevation is attempted.
	ErrLayoutAdminPINUnset = errors.New("layout_admin_pin_unset")

	// ErrLayoutAdminPINMismatch is returned by VerifyAdminPIN when
	// the caller-supplied PIN doesn't match the stored digest.
	// AuthService.Sudo translates it to its own `invalid_pin`
	// rejection so brute-force counters stay in one place.
	ErrLayoutAdminPINMismatch = errors.New("layout_admin_pin_mismatch")

	// ErrLayoutForbidden is returned when a non-admin attempts a
	// layout catalogue mutation guarded by CanManageLayouts.
	ErrLayoutForbidden = errors.New("forbidden")
)

// maxLayoutNameLen caps the human label so it fits the login
// dropdown without truncation acrobatics. A modeling event name in
// the wild rarely exceeds two words.
const maxLayoutNameLen = 64

// Layout admin PIN policy (§7a.7). Numeric, 4–8 digits — short
// enough to type from memory on a phone, long enough to make
// random guessing infeasible together with the rate limiter.
const (
	minLayoutAdminPINLength = 4
	maxLayoutAdminPINLength = 8
)

// SystemLayoutDefaultAdminPIN is the well-known admin PIN seeded
// onto a freshly-created system layout. Logged on first boot
// alongside the bootstrap admin account so the operator can use
// the sudo flow immediately. SHOULD be rotated through the layout
// settings dialog after first login.
const SystemLayoutDefaultAdminPIN = "0000"

// LayoutService implements the CRUD + lifecycle rules described in
// §3a.1 and §4.1 for Layout (Polish: makieta). It is intentionally
// agnostic of HTTP — handlers receive plain Go inputs and unpack
// errors via errors.Is.
//
// The service is also responsible for seeding the bootstrap system
// layout on a freshly-created database via EnsureSystemLayout, which
// runs out of the CLI startup sequence right after migrations.
type LayoutService struct {
	layouts               *repo.Layouts
	interlockings         *repo.Interlockings
	layoutInterlockings   *repo.LayoutInterlockings
	commandStations       *repo.CommandStations
	layoutCommandStations *repo.LayoutCommandStations
	sec                   security.LayoutSecurityContext
}

// NewLayoutService constructs a service bound to a Layouts
// repository.
func NewLayoutService(
	layouts *repo.Layouts,
	interlockings *repo.Interlockings,
	layoutInterlockings *repo.LayoutInterlockings,
	commandStations *repo.CommandStations,
	layoutCommandStations *repo.LayoutCommandStations,
) *LayoutService {
	return &LayoutService{
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
// argon2id digest of SystemLayoutDefaultAdminPIN so the sudo flow
// has a comparable hash on day one.
//
// Returns true when the seed actually happened so the caller can
// emit a one-shot log line.
func (s *LayoutService) EnsureSystemLayout(ctx context.Context) (bool, error) {
	if _, err := s.layouts.FindSystem(ctx); err == nil {
		return false, nil
	} else if !errors.Is(err, repo.ErrLayoutNotFound) {
		return false, err
	}

	hash, err := HashPIN(SystemLayoutDefaultAdminPIN)
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
func (s *LayoutService) ListAll(ctx context.Context) ([]domain.Layout, error) {
	return s.layouts.ListAll(ctx)
}

// ListSelectable returns the rows the login dropdown should offer:
// every non-locked layout (§7a.1).
func (s *LayoutService) ListSelectable(ctx context.Context) ([]domain.Layout, error) {
	return s.layouts.ListSelectable(ctx)
}

// Get looks a layout up by id. Translates ErrLayoutNotFound from the
// repo into the service-level sentinel of the same name (different
// package).
func (s *LayoutService) Get(ctx context.Context, id uint) (domain.Layout, error) {
	layout, err := s.layouts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrLayoutNotFound) {
			return domain.Layout{}, ErrLayoutNotFound
		}
		return domain.Layout{}, err
	}
	return layout, nil
}

// GetSystem returns the bootstrap system layout. EnsureSystemLayout
// guarantees the row exists from the very first server boot, so an
// ErrLayoutNotFound here would be a serious database-level fault.
func (s *LayoutService) GetSystem(ctx context.Context) (domain.Layout, error) {
	layout, err := s.layouts.FindSystem(ctx)
	if err != nil {
		if errors.Is(err, repo.ErrLayoutNotFound) {
			return domain.Layout{}, ErrLayoutNotFound
		}
		return domain.Layout{}, err
	}
	return layout, nil
}

// CreateInput is the validated payload of LayoutService.Create.
type CreateInput struct {
	Name              string
	CreatedBy         uint
	InterlockingIDs   []uint
	CommandStationIDs []uint
	// AdminPIN is the layout's initial sudo PIN (§7a.7). Empty
	// means "seed with SystemLayoutDefaultAdminPIN" — same UX as
	// the system layout, so a freshly-created layout already has
	// a usable PIN until the admin rotates it. Must satisfy
	// validateLayoutAdminPIN when non-empty.
	AdminPIN string
}

// Create inserts a brand-new non-system layout. Name uniqueness and
// non-emptiness are enforced explicitly so the HTTP layer can return
// the matching 4xx code without parsing SQL error strings.
func (s *LayoutService) Create(ctx context.Context, eff domain.EffectiveRoles, in CreateInput) (domain.Layout, error) {
	if err := s.checkManageLayouts(eff); err != nil {
		return domain.Layout{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return domain.Layout{}, ErrLayoutNameRequired
	}
	if len(name) > maxLayoutNameLen {
		return domain.Layout{}, ErrLayoutNameRequired
	}
	// The literal Name `default` is reserved for the system row. A
	// non-system layout that picks the same name would either fail
	// the unique constraint or shadow the system row through the
	// `name` lookup — both are confusing. Reject early.
	if name == domain.SystemLayoutName {
		return domain.Layout{}, ErrLayoutNameTaken
	}

	if _, err := s.layouts.FindByName(ctx, name); err == nil {
		return domain.Layout{}, ErrLayoutNameTaken
	} else if !errors.Is(err, repo.ErrLayoutNotFound) {
		return domain.Layout{}, err
	}

	pin := in.AdminPIN
	if pin == "" {
		pin = SystemLayoutDefaultAdminPIN
	}
	if err := validateLayoutAdminPIN(pin); err != nil {
		return domain.Layout{}, err
	}
	hash, err := HashPIN(pin)
	if err != nil {
		return domain.Layout{}, err
	}

	now := time.Now().UTC()
	layout := domain.Layout{
		Name:         name,
		IsSystem:     false,
		Locked:       false,
		CreatedBy:    in.CreatedBy,
		AdminPINHash: hash,
		CreatedAt:    now,
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
	return layout, nil
}

// validateLayoutAdminPIN enforces the digit / length policy on a
// candidate layout admin PIN (§7a.7). Returns ErrLayoutAdminPINInvalid
// for any input that fails the check.
func validateLayoutAdminPIN(pin string) error {
	if len(pin) < minLayoutAdminPINLength || len(pin) > maxLayoutAdminPINLength {
		return ErrLayoutAdminPINInvalid
	}
	for _, r := range pin {
		if r < '0' || r > '9' {
			return ErrLayoutAdminPINInvalid
		}
	}
	return nil
}

// UpdateAdminPIN rotates the layout's admin PIN. The empty string is
// treated as "no change" so a layout edit dialog that always submits
// the field doesn't accidentally clobber the digest with an empty
// hash. The system layout's PIN is rotatable too — the only
// immutable field on the system row is the Name (§7a.1).
//
// The HTTP layer guards the call with the policy that "only a
// non-sudo permanent admin may rotate the layout admin PIN" — see
// §7a.3 / §7a.7.
func (s *LayoutService) UpdateAdminPIN(ctx context.Context, eff domain.EffectiveRoles, id uint, newPIN string) (domain.Layout, error) {
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
	if err := validateLayoutAdminPIN(newPIN); err != nil {
		return domain.Layout{}, err
	}
	hash, err := HashPIN(newPIN)
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
//   - ErrLayoutAdminPINUnset — the layout has no PIN yet (an
//     empty digest never matches anything);
//   - ErrLayoutAdminPINMismatch — wrong PIN (the rate-limiter must
//     count this towards the brute-force counter);
//   - ErrLayoutNotFound — layout id is unknown.
func (s *LayoutService) VerifyAdminPIN(ctx context.Context, id uint, pin string) error {
	layout, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if layout.AdminPINHash == "" {
		return ErrLayoutAdminPINUnset
	}
	if err := verifyPIN(pin, layout.AdminPINHash); err != nil {
		return ErrLayoutAdminPINMismatch
	}
	return nil
}

// Rename updates the layout's Name. The system row rejects with
// ErrSystemLayoutImmutable so the UI can keep its row read-only.
func (s *LayoutService) Rename(ctx context.Context, eff domain.EffectiveRoles, id uint, newName string) (domain.Layout, error) {
	if err := s.checkManageLayouts(eff); err != nil {
		return domain.Layout{}, err
	}
	layout, err := s.Get(ctx, id)
	if err != nil {
		return domain.Layout{}, err
	}
	if layout.IsSystem {
		return domain.Layout{}, ErrSystemLayoutImmutable
	}
	name := strings.TrimSpace(newName)
	if name == "" {
		return domain.Layout{}, ErrLayoutNameRequired
	}
	if len(name) > maxLayoutNameLen {
		return domain.Layout{}, ErrLayoutNameRequired
	}
	if name == domain.SystemLayoutName {
		return domain.Layout{}, ErrLayoutNameTaken
	}
	if name == layout.Name {
		return layout, nil
	}

	if other, err := s.layouts.FindByName(ctx, name); err == nil {
		if other.ID != layout.ID {
			return domain.Layout{}, ErrLayoutNameTaken
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
// ErrSystemLayoutUndeletable (§4.1 DELETE).
//
// Note: §4.1 also says deletion must 409 when any live drive
// session is still pinned to the layout. Drive sessions are part of
// a later milestone (M4 command-station bring-up), so the check
// will be added together with `DriveSession`. The system row guard
// covers the most-important footgun for now.
func (s *LayoutService) Delete(ctx context.Context, eff domain.EffectiveRoles, id uint) error {
	if err := s.checkManageLayouts(eff); err != nil {
		return err
	}
	layout, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if layout.IsSystem {
		return ErrSystemLayoutUndeletable
	}
	return s.layouts.Delete(ctx, &layout)
}

// SetLocked toggles the Locked flag. Idempotent on either branch.
// Returns the updated row so the caller can echo the new state in
// the HTTP response.
func (s *LayoutService) SetLocked(ctx context.Context, id uint, locked bool) (domain.Layout, error) {
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
func (s *LayoutService) ListInterlockings(ctx context.Context, layoutID uint) ([]domain.Interlocking, error) {
	if _, err := s.Get(ctx, layoutID); err != nil {
		return nil, err
	}
	ids, err := s.layoutInterlockings.InterlockingIDsForLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	return s.interlockings.ListByIDs(ctx, ids)
}

// ListCommandStations returns command stations attached to a layout.
// For the system layout the live catalogue is synthesised (§4.1).
func (s *LayoutService) ListCommandStations(ctx context.Context, layoutID uint) ([]domain.CommandStation, error) {
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
// ErrCommandStationNotFound. The system layout rejects with
// ErrSystemLayoutCommandStationsImmutable.
func (s *LayoutService) SetCommandStations(ctx context.Context, eff domain.EffectiveRoles, layoutID, addedBy uint, commandStationIDs []uint) ([]domain.CommandStation, error) {
	if err := s.checkManageLayouts(eff); err != nil {
		return nil, err
	}
	layout, err := s.Get(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	if layout.IsSystem {
		return nil, ErrSystemLayoutCommandStationsImmutable
	}
	if err := s.setCommandStations(ctx, layoutID, addedBy, commandStationIDs); err != nil {
		return nil, err
	}
	return s.ListCommandStations(ctx, layoutID)
}

// SetInterlockings replaces the entire interlocking whitelist for a
// layout with the supplied id set. Unknown ids reject with
// ErrInterlockingNotFound. Duplicate ids in the input are ignored.
func (s *LayoutService) SetInterlockings(ctx context.Context, eff domain.EffectiveRoles, layoutID, addedBy uint, interlockingIDs []uint) ([]domain.Interlocking, error) {
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

func (s *LayoutService) checkManageLayouts(eff domain.EffectiveRoles) error {
	decision := s.sec.CanManageLayouts(eff)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case "forbidden":
		return ErrLayoutForbidden
	default:
		return errors.New(decision.Reason)
	}
}

func (s *LayoutService) setCommandStations(ctx context.Context, layoutID, addedBy uint, commandStationIDs []uint) error {
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
				return ErrCommandStationNotFound
			}
			return err
		}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return ErrLayoutNeedsAtLeastOneCommandStation
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

func (s *LayoutService) setInterlockings(ctx context.Context, layoutID, addedBy uint, interlockingIDs []uint) error {
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
				return ErrInterlockingNotFound
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
// ErrLayoutLocked. Used by AuthService.Login to fulfil §7a.1 steps
// 2 + 3 without coupling AuthService to the Layouts repo directly.
func (s *LayoutService) ValidateForLogin(ctx context.Context, id uint) (domain.Layout, error) {
	layout, err := s.Get(ctx, id)
	if err != nil {
		return domain.Layout{}, err
	}
	if layout.Locked {
		return domain.Layout{}, ErrLayoutLocked
	}
	return layout, nil
}
