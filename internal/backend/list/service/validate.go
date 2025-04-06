package service

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/samber/lo"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

func (s *Service) validate(model list.ProductList) error {
	err := validator.New().Struct(model)
	if err != nil {
		return fmt.Errorf("%w: %w", myerr.ErrInvalidArgument, err)
	}

	if len(lo.SliceToMap(model.Members, func(item list.Member) (id.ID[user.User], struct{}) {
		return item.UserID, struct{}{}
	})) != len(model.Members) {
		return fmt.Errorf("%w: non-unique members", myerr.ErrInvalidArgument)
	}

	if len(lo.SliceToMap(model.States, func(item list.ProductState) (id.ID[product.Product], struct{}) {
		return item.Product.ID, struct{}{}
	})) != len(model.States) {
		return fmt.Errorf("%w: non-unique product states", myerr.ErrInvalidArgument)
	}

	return nil
}
