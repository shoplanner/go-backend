package service

import (
	"fmt"
	"slices"

	"go-backend/internal/backend/shopmap"
	"go-backend/pkg/myerr"
)

func (s *Service) validate(shopMap shopmap.ShopMap) error {
	if slices.Contains(shopMap.ViewerIDList, shopMap.OwnerID) {
		return fmt.Errorf("%w: user-owner can't be viewer", myerr.ErrInvalidArgument)
	}

	if err := s.validator.Struct(shopMap); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	return nil
}
