package service

import (
	"go-backend/internal/backend/list"

	"github.com/go-playground/validator/v10"
)

func (s *Service) validate(model list.ProductList) error {
	return validator.New().Struct(model)
}
