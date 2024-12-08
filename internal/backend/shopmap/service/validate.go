package service

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"go-backend/internal/backend/shopmap"
)

func (s *Service) validate(ctx context.Context, shopMap shopmap.ShopMap) error {
	if err := s.validator.StructCtx(ctx, shopMap); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	return nil
}

func (s *Service) initValidator() {
	s.validator.RegisterValidationCtx("user_id_valid", s.checkUserExist)
}

func (s *Service) checkUserExist(ctx context.Context, fl validator.FieldLevel) bool {
	uuid, err := uuid.Parse(fl.Field().String())
	if err != nil {
		return false
	}
	return s.users.IsExists(ctx, uuid) == nil
}
