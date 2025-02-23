package service

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	"github.com/samber/mo"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
	"go-backend/pkg/ph"
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
					MemberOptions: list.MemberOptions{
						UserID: ownerID,
						Role:   list.MemberTypeOwner,
					},
					UserName:  "",
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

func (s *Service) DeleteList(ctx context.Context, userID id.ID[user.User], listID id.ID[list.ProductList]) error {
	err := s.repo.GetAndDeleteList(ctx, listID, func(oldList list.ProductList) error {
		return oldList.CheckRole(userID, list.MemberTypeEditor)
	})
	if err != nil {
		return fmt.Errorf("can't delete product list %s: %w", listID, err)
	}

	return nil
}

func (s *Service) GetByID(ctx context.Context, listID id.ID[list.ProductList], userID id.ID[user.User]) (list.ProductList, error) {
	model, err := s.repo.GetByListID(ctx, listID)
	if err != nil {
		return list.ProductList{}, fmt.Errorf("can't get list %s from storage: %w", listID, err)
	}

	if err := model.CheckRole(userID, list.MemberTypeViewer); err != nil {
		return list.ProductList{}, err
	}

	return model, nil
}

func (s *Service) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]list.ProductList, error) {
	models, err := s.repo.GetListMetaByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("can't get disk meta related to user %s: %w", userID, err)
	}

	return models, nil
}

func (s *Service) AppendMembers(ctx context.Context, listID id.ID[list.ProductList], userID id.ID[user.User], members []list.MemberOptions) (list.ProductList, error) {
	model, err := s.repo.GetAndUpdate(ctx, listID, func(oldList list.ProductList) (list.ProductList, error) {
		if err := oldList.CheckRole(userID, list.MemberTypeAdmin); err != nil {
			return oldList, err
		}

		oldList.Members = append(oldList.Members, lo.Map(members, func(item list.MemberOptions, _ int) list.Member {
			return list.Member{
				MemberOptions: item,
				CreatedAt:     date.NewCreateDate[list.Member](),
				UpdatedAt:     date.NewUpdateDate[list.Member](),
			}
		})...)

		if err := s.validate(oldList); err != nil {
			return oldList, err
		}

		return oldList, nil
	})
	if err != nil {
		return list.ProductList{}, fmt.Errorf("can't update list %s: %w", listID, err)
	}

	return model, nil
}

func (s *Service) AppendProducts(
	ctx context.Context,
	listID id.ID[list.ProductList],
	userID id.ID[user.User],
	states map[id.ID[product.Product]]list.ProductStateOptions,
) (
	list.ProductList,
	error,
) {
	model, err := s.repo.GetAndUpdate(ctx, listID, func(oldList list.ProductList) (list.ProductList, error) {
		if err := oldList.CheckRole(userID, list.MemberTypeEditor); err != nil {
			return oldList, err
		}

		newList := oldList.Clone()

		for productID, stateOpts := range states {
			newProductState := list.ProductState{
				ProductStateOptions: stateOpts,
				Product: product.Product{
					Options: product.Options{
						Name:     "",
						Category: mo.None[product.Category](),
						Forms:    []product.Form{},
					},
					ID:        productID,
					CreatedAt: date.CreateDate[product.Product]{},
					UpdatedAt: date.UpdateDate[product.Product]{},
				},
				CreatedAt: date.NewCreateDate[list.ProductState](),
				UpdatedAt: date.NewUpdateDate[list.ProductState](),
			}

			oldList.States = append(oldList.States, newProductState)
		}

		if err := s.validate(oldList); err != nil {
			return oldList, err
		}

		return newList, nil
	})
	if err != nil {
		return list.ProductList{}, fmt.Errorf("can't append products: %w", err)
	}

	return model, nil
}

func (s *Service) DeleteProducts(
	ctx context.Context,
	listID id.ID[list.ProductList],
	userID id.ID[user.User],
	toDelete []id.ID[product.Product],
) (
	list.ProductList,
	error,
) {
	model, err := s.repo.GetAndUpdate(ctx, listID, func(oldList list.ProductList) (list.ProductList, error) {
		if err := oldList.CheckRole(userID, list.MemberTypeEditor); err != nil {
			return oldList, err
		}

		newList := oldList.Clone()

		currentStates := lo.SliceToMap(newList.States, func(item list.ProductState) (id.ID[product.Product], list.ProductState) {
			return item.Product.ID, item
		})

		for _, productID := range toDelete {
			if _, found := currentStates[productID]; !found {
				return oldList, fmt.Errorf("%w: state with product id %s", myerr.ErrNotFound, productID)
			}

			delete(currentStates, productID)
		}

		if err := s.validate(newList); err != nil {
			return oldList, err
		}

		return newList, nil
	})
	if err != nil {
		return list.ProductList{}, fmt.Errorf("can't delete products from list %s: %w", listID, err)
	}

	return model, nil
}
