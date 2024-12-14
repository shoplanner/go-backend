package repo

import (
	"context"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"go-backend/internal/backend/favorite"
)

type Repo struct {
	col *mongo.Collection
}

func NewRepo(db *mongo.Database) *Repo {
	return &Repo{col: db.Collection("favorites")}
}

func (r *Repo) UserID(ctx context.Context, userID uuid.UUID) (favorite.List, error) {
	var model favorite.List
	return model, r.col.FindOne(ctx, bson.D{{Key: "_id", Value: userID}}).Decode(&model)
}

func (r *Repo) GetAndModify(ctx context.Context, userID uuid.UUID, modifyFunc func(ctx context.Context, list favorite.List) (favorite.List, error)) (favorite.List, error) {
	var model favorite.List

	session, err := r.col.Database().Client().StartSession()
	if err != nil {
		return model, err
	}
	defer session.EndSession(ctx)

	list, err := session.WithTransaction(ctx, func(ctx mongo.SessionContext) (interface{}, error) {
		list, getError := r.UserID(ctx, userID)
		if getError != nil {
			return list, getError
		}

		return modifyFunc(ctx, list)
	})

	return list.(favorite.List), err
}
