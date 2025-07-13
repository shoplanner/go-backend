package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/samber/mo"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

type favoritesRepo interface {
	CreateList(context.Context, favorite.List) error
	GetByID(context.Context, id.ID[favorite.List]) (favorite.List, error)
	GetByUserID(context.Context, id.ID[user.User]) ([]favorite.List, error)
	GetAndUpdate(context.Context, id.ID[favorite.List], func(list favorite.List) (favorite.List, error)) (
		favorite.List,
		error,
	)
}

type users interface {
	RegisterSubscriber(user.Subscriber)
}

// Service is the service
type Service struct {
	repo favoritesRepo
}

func NewService(repo favoritesRepo, users users) *Service {
	s := &Service{repo: repo}
	users.RegisterSubscriber(s)
	return s
}

func (s *Service) AddProducts(
	ctx context.Context,
	listID id.ID[favorite.List],
	userID id.ID[user.User],
	productIDs []id.ID[product.Product],
) (
	favorite.List,
	error,
) {
	fmt.Println(productIDs)
	model, err := s.repoGetAndUpdate(ctx, listID, func(list favorite.List) (favorite.List, error) {
		if err := list.AllowedToEdit(userID); err != nil {
			return favorite.List{}, fmt.Errorf("user %s is not allowed to edit list %s: %w", userID, listID, err)
		}
		for _, productID := range productIDs {
			list.Products = append(list.Products, favorite.Favorite{
				Product: product.Product{
					Options:   product.Options{Name: "", Category: mo.None[product.Category](), Forms: []product.Form{}},
					ID:        productID,
					CreatedAt: date.CreateDate[product.Product]{Time: time.Time{}},
					UpdatedAt: date.UpdateDate[product.Product]{Time: time.Time{}},
				},
				CreatedAt: date.NewCreateDate[favorite.Favorite](),
				UpdatedAt: date.NewUpdateDate[favorite.Favorite](),
			})
		}
		return list, nil
	})
	if err != nil {
		return model, err
	}

	return model, nil
}

func (s *Service) DeleteProducts(ctx context.Context,
	listID id.ID[favorite.List],
	userID id.ID[user.User],
	productIDs []id.ID[product.Product],
) (
	favorite.List,
	error,
) {
	model, err := s.repoGetAndUpdate(ctx, listID, func(list favorite.List) (favorite.List, error) {
		if err := list.AllowedToEdit(userID); err != nil {
			return favorite.List{}, fmt.Errorf("user %s is not allowed to edit list %s: %w", userID, listID, err)
		}

		productExists := make(map[string]favorite.Favorite, len(list.Products))
		toDelete := make(map[string]struct{}, len(productIDs))
		for _, p := range list.Products {
			productExists[p.Product.ID.String()] = p
		}
		for _, productID := range productIDs {
			toDelete[productID.String()] = struct{}{}
		}

		for _, productID := range productIDs {
			if _, exists := productExists[productID.String()]; !exists {
				return list, favorite.ErrProductNotFound(listID, productID)
			}
		}

		list.Products = slices.DeleteFunc(list.Products, func(product favorite.Favorite) bool {
			_, kek := toDelete[product.Product.ID.String()]
			return kek
		})

		return list, nil
	})
	if err != nil {
		return model, err
	}

	return model, nil
}

func (s *Service) GetListByID(
	ctx context.Context,
	listID id.ID[favorite.List],
	userID id.ID[user.User],
) (
	favorite.List,
	error,
) {
	model, err := s.repo.GetByID(ctx, listID)
	if err != nil {
		return favorite.List{}, fmt.Errorf("can't get list %s: %w", listID, err)
	}

	if err = model.AllowedToView(userID); err != nil {
		return favorite.List{}, fmt.Errorf("user %s is not allowed to view list %s: %w", userID, listID, err)
	}

	return model, nil
}

func (s *Service) GetListsByUserID(ctx context.Context, userID id.ID[user.User]) ([]favorite.List, error) {
	models, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("can't get favorites lists of user %s: %w", userID, err)
	}

	return models, nil
}

func (s *Service) repoGetAndUpdate(
	ctx context.Context,
	listID id.ID[favorite.List],
	updateFunc func(favorite.List) (favorite.List, error),
) (
	favorite.List,
	error,
) {
	model, err := s.repo.GetAndUpdate(ctx, listID, updateFunc)
	if err != nil {
		return model, fmt.Errorf("can't update repo: %w", err)
	}
	return model, nil
}

func (s *Service) createList(ctx context.Context, list favorite.List) error {
	if err := s.repo.CreateList(ctx, list); err != nil {
		return fmt.Errorf("can't save new list to storage: %w", err)
	}

	return nil
}
