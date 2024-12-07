package service

import (
	"context"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

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
