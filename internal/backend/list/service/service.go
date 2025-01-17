package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

type repo interface {
	CreateList(context.Context, list.ProductList) error
	GetByListID(context.Context, id.ID[list.ProductList]) (list.ProductList, error)
	GetListMetaByUserID(context.Context, id.ID[user.User]) ([]list.ProductList, error)
	GetAndUpdate(
		context.Context,
		id.ID[list.ProductList],
		func(list.ProductList) (list.ProductList, error),
	) (
		list.ProductList,
		error,
	)
	GetAndDeleteList(context.Context, id.ID[list.ProductList], func(list.ProductList) error) error
}

type Service struct {
	repo repo
}

func NewService(repo repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, ownerID id.ID[user.User], options list.Options) (list.ProductList, error) {
	newList := list.ProductList{
		Options: list.Options{
			States: []list.ProductState{},
			Members: []list.Member{
				{
					UserID:    ownerID,
					UserName:  "",
					Role:      list.MemberTypeOwner,
					CreatedAt: date.NewCreateDate[list.Member](),
					UpdatedAt: date.NewUpdateDate[list.Member](),
				},
			},
			Status: list.ExecStatusPlanning,
			Title:  options.Title,
		},
		ID:        id.NewID[list.ProductList](),
		CreatedAt: date.NewCreateDate[list.ProductList](),
		UpdatedAt: date.NewUpdateDate[list.ProductList](),
	}

	if err := s.validate(newList); err != nil {
		return list.ProductList{}, err
	}

	if err := s.repo.CreateList(ctx, newList); err != nil {
		return list.ProductList{}, fmt.Errorf("can't create new list: %w", err)
	}

	return newList, nil
}

func (s *Service) Update(ctx context.Context, listID id.ID[list.ProductList], options list.Options) (list.ProductList, error) {
	model, err := s.repo.GetAndUpdate(ctx, listID, func(oldList list.ProductList) (list.ProductList, error) {
		oldList.Options = options

		if err := s.validate(oldList); err != nil {
			return list.ProductList{}, err
		}

		return oldList, nil
	})
	if err != nil {
		return model, fmt.Errorf("can't update list %s: %w", err)
	}

	return model, nil
}

func (s *Service) DeleteList(ctx context.Context, listID id.ID[list.ProductList]) error {
}

func (s *Service) GetByID(ctx context.Context, listID id.ID[list.ProductList], userID id.ID[user.User]) (list.ProductList, error) {
	model, err := s.repo.GetByListID(ctx, listID)
	if err != nil {
		return model, fmt.Errorf("can't get list %s from storage: %w", listID, err)
	}
}

func (s *Service) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]list.ProductList, error) {
}

func (s *Service) AppendMembers(ctx context.Context, listID id.ID[list.ProductList], []list.MemberOptions) (list.ProductList, error) {

}

func (s *Service) Append
