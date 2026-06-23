package users

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListRoles(ctx context.Context) ([]Role, error) {
	return s.repo.ListRoles(ctx)
}

func (s *Service) ListUserRoles(ctx context.Context, userID int) ([]Role, error) {
	if _, err := s.repo.GetUserByID(ctx, userID); err != nil {
		return nil, err
	}
	return s.repo.ListUserRoles(ctx, userID)
}

func (s *Service) AssignRole(ctx context.Context, userID, roleID int) ([]Role, error) {
	if _, err := s.repo.GetUserByID(ctx, userID); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetRoleByID(ctx, roleID); err != nil {
		return nil, err
	}
	if err := s.repo.AssignRole(ctx, userID, roleID); err != nil {
		return nil, err
	}
	return s.repo.ListUserRoles(ctx, userID)
}

func (s *Service) RevokeRole(ctx context.Context, userID, roleID int) ([]Role, error) {
	if _, err := s.repo.GetUserByID(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.repo.RevokeRole(ctx, userID, roleID); err != nil {
		return nil, err
	}
	return s.repo.ListUserRoles(ctx, userID)
}
