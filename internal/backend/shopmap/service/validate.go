package service

import (
	"errors"
	"fmt"
	"slices"

	"go-backend/internal/backend/shopmap"
)

func (s *Service) validate(shopMap shopmap.ShopMap) error {
	if slices.Contains(shopMap.ViewerIDList, shopMap.OwnerID) {
		return errors.New("user-owner can't be viewer")
	}

	if err := s.validator.Struct(shopMap); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	return nil
}
