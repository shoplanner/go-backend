package shopmap

import "github.com/google/uuid"

type ShopMapRequest struct {
	Categories []string `bson:"categories"`
}

type ShopMapResponse struct {
	ShopMapRequest `bson:"inline"`

	UserID uuid.UUID `json:"user_id" bson:"user_id"`
	ID     uuid.UUID `json:"id" bson:"_id"`
}
