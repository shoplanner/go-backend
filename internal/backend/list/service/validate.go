package service

import (
	"fmt"

	"github.com/go-playground/validator/v10"

	"go-backend/internal/backend/list"
	"go-backend/pkg/myerr"
)

func (s *Service) validate(model list.ProductList) error {
	err := validator.New().Struct(model)
	if err != nil {
		return fmt.Errorf("%w: %w", myerr.ErrInvalidArgument, err)
	}

	return nil
}
