package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

// Interlocking implements CRUD over the interlocking catalogue.
type Interlocking struct {
	interlockings       *repo.Interlockings
	layoutInterlockings *repo.LayoutInterlockings
	sec                 security.InterlockingSecurityContext
}

func NewInterlocking(
	interlockings *repo.Interlockings,
	layoutInterlockings *repo.LayoutInterlockings,
) *Interlocking {
	return &Interlocking{
		interlockings:       interlockings,
		layoutInterlockings: layoutInterlockings,
	}
}

func (s *Interlocking) ListAll(ctx context.Context) ([]domain.Interlocking, error) {
	return s.interlockings.ListAll(ctx)
}

func (s *Interlocking) ListForLayout(ctx context.Context, layoutID uint) ([]domain.Interlocking, error) {
	ids, err := s.layoutInterlockings.InterlockingIDsForLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	return s.interlockings.ListByIDs(ctx, ids)
}

func (s *Interlocking) Get(ctx context.Context, id uint) (domain.Interlocking, error) {
	row, err := s.interlockings.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrInterlockingNotFound) {
			return domain.Interlocking{}, svcerrors.ErrInterlockingNotFound
		}
		return domain.Interlocking{}, err
	}
	return row, nil
}

type InterlockingCreateInput struct {
	Name     string
	Location string
}

func (s *Interlocking) Create(ctx context.Context, eff domain.EffectiveRoles, in InterlockingCreateInput) (domain.Interlocking, error) {
	if err := s.checkCatalogManage(eff); err != nil {
		return domain.Interlocking{}, err
	}
	name, err := validation.SanitiseInterlockingName(in.Name)
	if err != nil {
		return domain.Interlocking{}, err
	}
	location := validation.SanitiseInterlockingLocation(in.Location)

	if _, err := s.interlockings.FindByName(ctx, name); err == nil {
		return domain.Interlocking{}, svcerrors.ErrInterlockingNameTaken
	} else if !errors.Is(err, repo.ErrInterlockingNotFound) {
		return domain.Interlocking{}, err
	}

	row := domain.Interlocking{
		Name:      name,
		Location:  location,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.interlockings.Insert(ctx, &row); err != nil {
		return domain.Interlocking{}, err
	}
	return row, nil
}

type InterlockingUpdateInput struct {
	Name     *string
	Location *string
}

func (s *Interlocking) Update(ctx context.Context, eff domain.EffectiveRoles, id uint, in InterlockingUpdateInput) (domain.Interlocking, error) {
	if err := s.checkCatalogManage(eff); err != nil {
		return domain.Interlocking{}, err
	}
	row, err := s.Get(ctx, id)
	if err != nil {
		return domain.Interlocking{}, err
	}

	if in.Name != nil {
		name, err := validation.SanitiseInterlockingName(*in.Name)
		if err != nil {
			return domain.Interlocking{}, err
		}
		if name != row.Name {
			if other, err := s.interlockings.FindByName(ctx, name); err == nil {
				if other.ID != row.ID {
					return domain.Interlocking{}, svcerrors.ErrInterlockingNameTaken
				}
			} else if !errors.Is(err, repo.ErrInterlockingNotFound) {
				return domain.Interlocking{}, err
			}
			row.Name = name
		}
	}
	if in.Location != nil {
		row.Location = validation.SanitiseInterlockingLocation(*in.Location)
	}

	if err := s.interlockings.Update(ctx, &row); err != nil {
		return domain.Interlocking{}, err
	}
	return row, nil
}

func (s *Interlocking) Delete(ctx context.Context, eff domain.EffectiveRoles, id uint) error {
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

func (s *Interlocking) checkCatalogManage(eff domain.EffectiveRoles) error {
	decision := s.sec.CanManageCatalog(eff)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case security.ReasonForbidden:
		return svcerrors.ErrInterlockingForbidden
	default:
		return errors.New(decision.Reason)
	}
}
