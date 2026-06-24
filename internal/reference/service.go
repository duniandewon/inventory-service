package reference

import (
	"context"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func normalizeUnitName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrEmptyUnitName
	}
	return name, nil
}

func (s *Service) ListUnits(ctx context.Context) ([]UnitOfMeasure, error) {
	return s.repo.ListUnits(ctx)
}

func (s *Service) CreateUnit(ctx context.Context, name string) (*UnitOfMeasure, error) {
	name, err := normalizeUnitName(name)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateUnit(ctx, name)
}

func (s *Service) UpdateUnit(ctx context.Context, id int, name string) (*UnitOfMeasure, error) {
	name, err := normalizeUnitName(name)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateUnit(ctx, id, name)
}

func (s *Service) DeleteUnit(ctx context.Context, id int) error {
	if _, err := s.repo.GetUnitByID(ctx, id); err != nil {
		return err
	}

	inUse, err := s.repo.IsUnitInUse(ctx, id)
	if err != nil {
		return err
	}
	if inUse {
		return ErrUnitInUse
	}

	return s.repo.DeleteUnit(ctx, id)
}
