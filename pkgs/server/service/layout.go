package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
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

	// ErrSystemLayoutCannotBeLocked is returned by SetLocked on the
	// IsSystem row.
	ErrSystemLayoutCannotBeLocked = errors.New("default_layout_cannot_be_locked")
)

// maxLayoutNameLen caps the human label so it fits the login
// dropdown without truncation acrobatics. A modeling event name in
// the wild rarely exceeds two words.
const maxLayoutNameLen = 64

// LayoutService implements the CRUD + lifecycle rules described in
// §3a.1 and §4.1 for Layout (Polish: makieta). It is intentionally
// agnostic of HTTP — handlers receive plain Go inputs and unpack
// errors via errors.Is.
//
// The service is also responsible for seeding the bootstrap system
// layout on a freshly-created database via EnsureSystemLayout, which
// runs out of the CLI startup sequence right after migrations.
type LayoutService struct {
	layouts *repo.Layouts
}

// NewLayoutService constructs a service bound to a Layouts
// repository.
func NewLayoutService(layouts *repo.Layouts) *LayoutService {
	return &LayoutService{layouts: layouts}
}

// EnsureSystemLayout inserts the bootstrap system layout row if it
// does not already exist. The operation is idempotent — calling it on
// every server startup is safe. The seeded row has
// Name = domain.SystemLayoutName, IsSystem = true, Locked = false,
// CreatedBy = 0 (no admin user owns it).
//
// Returns true when the seed actually happened so the caller can
// emit a one-shot log line.
func (s *LayoutService) EnsureSystemLayout(ctx context.Context) (bool, error) {
	if _, err := s.layouts.FindSystem(ctx); err == nil {
		return false, nil
	} else if !errors.Is(err, repo.ErrLayoutNotFound) {
		return false, err
	}

	now := time.Now().UTC()
	layout := domain.Layout{
		Name:      domain.SystemLayoutName,
		IsSystem:  true,
		Locked:    false,
		CreatedBy: 0,
		CreatedAt: now,
		UpdatedAt: now,
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
	Name      string
	CreatedBy uint
}

// Create inserts a brand-new non-system layout. Name uniqueness and
// non-emptiness are enforced explicitly so the HTTP layer can return
// the matching 4xx code without parsing SQL error strings.
//
// Note (§3a.1 / §4.1): in the full spec the request also carries
// `commandStationIds: [...]`. Command stations are not yet a thing in
// the codebase (they land in the milestone that introduces
// CommandStationService); when that lands, this constructor will
// gain a CommandStationIDs field with the matching ≥1 validation.
func (s *LayoutService) Create(ctx context.Context, in CreateInput) (domain.Layout, error) {
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

	now := time.Now().UTC()
	layout := domain.Layout{
		Name:      name,
		IsSystem:  false,
		Locked:    false,
		CreatedBy: in.CreatedBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.layouts.Insert(ctx, &layout); err != nil {
		return domain.Layout{}, err
	}
	return layout, nil
}

// Rename updates the layout's Name. The system row rejects with
// ErrSystemLayoutImmutable so the UI can keep its row read-only.
func (s *LayoutService) Rename(ctx context.Context, id uint, newName string) (domain.Layout, error) {
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
func (s *LayoutService) Delete(ctx context.Context, id uint) error {
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
	if layout.IsSystem && locked {
		return domain.Layout{}, ErrSystemLayoutCannotBeLocked
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
