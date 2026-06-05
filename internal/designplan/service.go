package designplan

import (
	"context"
	"strings"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error) {
	return s.repo.ListStores(ctx, filters)
}

func (s *Service) GetStore(ctx context.Context, id int64) (*Store, error) {
	return s.repo.GetStore(ctx, id)
}

func (s *Service) CreateStore(ctx context.Context, input StoreInput) (*Store, error) {
	if err := ValidateStoreInput(input); err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	return s.repo.CreateStore(ctx, input)
}

func (s *Service) UpdateStore(ctx context.Context, id int64, input StoreInput) (*Store, error) {
	if err := ValidateStoreInput(input); err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	return s.repo.UpdateStore(ctx, id, input)
}

func (s *Service) DeleteStore(ctx context.Context, id int64) error {
	return s.repo.DeleteStore(ctx, id)
}

func (s *Service) CheckDuplicate(ctx context.Context, request DuplicateCheckRequest) (DuplicateCheckResult, error) {
	if strings.TrimSpace(request.Name) == "" {
		return DuplicateCheckResult{}, &ValidationError{Fields: map[string]string{
			"name": "门店名必填",
		}}
	}
	return s.repo.CheckDuplicate(ctx, strings.TrimSpace(request.Name), request.ExcludeStoreID)
}
