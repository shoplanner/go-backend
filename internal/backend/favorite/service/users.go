package service

import (
	"context"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

func (s *Service) HandleUserCreated(ctx context.Context, newUser user.User) error {
	newList := favorite.List{
		ID: id.NewID[favorite.List](),
		Members: []favorite.Member{
			{
				UserID:    newUser.ID,
				Type:      favorite.MemberTypeOwner,
				CreatedAt: date.NewCreateDate[favorite.Member](),
				UpdatedAt: date.NewUpdateDate[favorite.Member](),
			},
		},
		CreatedAt: date.NewCreateDate[favorite.List](),
		UpdatedAt: date.NewUpdateDate[favorite.List](),
		Products:  []favorite.Favorite{},
		Type:      favorite.ListTypePersonal,
	}

	return s.createList(ctx, newList)
}
