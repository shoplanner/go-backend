package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"go-backend/internal/backend/shopmap"
)

type Repo struct {
	col *mongo.Collection
}

func New(c *mongo.Collection) *Repo {
	return &Repo{
		col: c,
	}
}

func (r *Repo) Get(ctx context.Context, id uuid.UUID) (shopmap.ShopMap, error) {
	var shopMap shopmap.ShopMap
	return shopMap, r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&shopMap)
}

func (r *Repo) GetByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]shopmap.ShopMap, error) {
	var shopMaps []shopmap.ShopMap
	cur, err := r.col.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return shopMaps, err
	}

	return shopMaps, cur.All(ctx, &shopMaps)
}

func (r *Repo) Create(ctx context.Context, shopMap shopmap.ShopMap) error {
	_, err := r.col.InsertOne(ctx, shopMap)
	return err
}

func (r *Repo) Update(ctx context.Context, shopMap shopmap.ShopMap) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": shopMap.ID}, shopMap)
	return err
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *Repo) GetAndUpdate(
	ctx context.Context,
	id uuid.UUID,
	updateFunc func(context.Context, shopmap.ShopMap) (shopmap.ShopMap, error),
) (shopmap.ShopMap, error) {
	session, err := r.col.Database().Client().StartSession()
	if err != nil {
		return shopmap.ShopMap{}, fmt.Errorf("can't start mongodb session: %w", err)
	}

	result, err := session.WithTransaction(ctx, func(ctx mongo.SessionContext) (interface{}, error) {
		shopMap, err := r.Get(ctx, id)
		if err != nil {
			return shopMap, err
		}

		updateMap, err := updateFunc(ctx, shopMap)
		if err != nil {
			return shopMap, err
		}

		return updateMap, r.Update(ctx, updateMap)
	})
	return result.(shopmap.ShopMap), err
}

func (r *Repo) GetByUserID(ctx context.Context, userID uuid.UUID) ([]shopmap.ShopMap, error) {
	var shopMaps []shopmap.ShopMap
	cur, err := r.col.Find(
		ctx,
		bson.M{"owner_id": userID, "viewers_id": bson.M{"$in": userID}},
	)
	if err != nil {
		return shopMaps, err
	}

	return shopMaps, cur.All(ctx, &shopMaps)
}
