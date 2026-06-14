package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

var (
	ErrInterlockingNotFound     = errors.New("interlocking_not_found")
	ErrInterlockingNameTaken    = errors.New("interlocking_name_taken")
	ErrInterlockingNameRequired = errors.New("interlocking_name_required")
	ErrInterlockingForbidden    = errors.New("forbidden")
)

const maxInterlockingNameLen = 64
const maxInterlockingLocationLen = 512

// InterlockingService implements CRUD over the interlocking catalogue.
type InterlockingService struct {
	interlockings       *repo.Interlockings
	layoutInterlockings *repo.LayoutInterlockings
	sec                 security.InterlockingSecurityContext
}

// NewInterlockingService constructs an InterlockingService.
func NewInterlockingService(
	interlockings *repo.Interlockings,
	layoutInterlockings *repo.LayoutInterlockings,
) *InterlockingService {
	return &InterlockingService{
		interlockings:       interlockings,
		layoutInterlockings: layoutInterlockings,
	}
}

// ListAll returns every interlocking in the catalogue (admin view).
func (s *InterlockingService) ListAll(ctx context.Context) ([]domain.Interlocking, error) {
	return s.interlockings.ListAll(ctx)
}

// ListForLayout returns interlockings whitelisted for the given layout.
func (s *InterlockingService) ListForLayout(ctx context.Context, layoutID uint) ([]domain.Interlocking, error) {
	ids, err := s.layoutInterlockings.InterlockingIDsForLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	return s.interlockings.ListByIDs(ctx, ids)
}

// Get looks an interlocking up by id.
func (s *InterlockingService) Get(ctx context.Context, id uint) (domain.Interlocking, error) {
	row, err := s.interlockings.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrInterlockingNotFound) {
			return domain.Interlocking{}, ErrInterlockingNotFound
		}
		return domain.Interlocking{}, err
	}
	return row, nil
}

// CreateInput is the validated payload of InterlockingService.Create.
type InterlockingCreateInput struct {
	Name     string
	Location string
}

// Create inserts a new interlocking into the catalogue. Authority is
// decided by InterlockingSecurityContext.CanManageCatalog (§7a.3).
func (s *InterlockingService) Create(ctx context.Context, eff domain.EffectiveRoles, in InterlockingCreateInput) (domain.Interlocking, error) {
	if err := s.checkCatalogManage(eff); err != nil {
		return domain.Interlocking{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > maxInterlockingNameLen {
		return domain.Interlocking{}, ErrInterlockingNameRequired
	}
	location := strings.TrimSpace(in.Location)
	if len(location) > maxInterlockingLocationLen {
		location = location[:maxInterlockingLocationLen]
	}

	if _, err := s.interlockings.FindByName(ctx, name); err == nil {
		return domain.Interlocking{}, ErrInterlockingNameTaken
	} else if !errors.Is(err, repo.ErrInterlockingNotFound) {
		return domain.Interlocking{}, err
	}

	now := time.Now().UTC()
	row := domain.Interlocking{
		Name:      name,
		Location:  location,
		CreatedAt: now,
	}
	if err := s.interlockings.Insert(ctx, &row); err != nil {
		return domain.Interlocking{}, err
	}
	return row, nil
}

// UpdateInput carries optional fields for InterlockingService.Update.
type InterlockingUpdateInput struct {
	Name     *string
	Location *string
}

// Update mutates an existing interlocking. Authority is decided by
// InterlockingSecurityContext.CanManageCatalog (§7a.3).
func (s *InterlockingService) Update(ctx context.Context, eff domain.EffectiveRoles, id uint, in InterlockingUpdateInput) (domain.Interlocking, error) {
	if err := s.checkCatalogManage(eff); err != nil {
		return domain.Interlocking{}, err
	}
	row, err := s.Get(ctx, id)
	if err != nil {
		return domain.Interlocking{}, err
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || len(name) > maxInterlockingNameLen {
			return domain.Interlocking{}, ErrInterlockingNameRequired
		}
		if name != row.Name {
			if other, err := s.interlockings.FindByName(ctx, name); err == nil {
				if other.ID != row.ID {
					return domain.Interlocking{}, ErrInterlockingNameTaken
				}
			} else if !errors.Is(err, repo.ErrInterlockingNotFound) {
				return domain.Interlocking{}, err
			}
			row.Name = name
		}
	}
	if in.Location != nil {
		location := strings.TrimSpace(*in.Location)
		if len(location) > maxInterlockingLocationLen {
			location = location[:maxInterlockingLocationLen]
		}
		row.Location = location
	}

	if err := s.interlockings.Update(ctx, &row); err != nil {
		return domain.Interlocking{}, err
	}
	return row, nil
}

// Delete removes an interlocking and every layout whitelist row
// pointing at it. Authority is decided by
// InterlockingSecurityContext.CanManageCatalog (§7a.3).
func (s *InterlockingService) Delete(ctx context.Context, eff domain.EffectiveRoles, id uint) error {
	if err := s.checkCatalogManage(eff); err != nil {
		return err
	}
	row, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.layoutInterlockings.DeleteAllForInterlocking(ctx, id); err != nil {
		return err
	}
	return s.interlockings.Delete(ctx, &row)
}

func (s *InterlockingService) checkCatalogManage(eff domain.EffectiveRoles) error {
	decision := s.sec.CanManageCatalog(eff)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case "forbidden":
		return ErrInterlockingForbidden
	default:
		return errors.New(decision.Reason)
	}
}
