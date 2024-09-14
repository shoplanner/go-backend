package repo

import (
	"context"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"go-backend/internal/shopmap/models"
)

type Repo struct {
	col *mongo.Collection
}

func New(c *mongo.Collection) *Repo {
	return &Repo{
		col: c,
	}
}

func (r *Repo) Get(ctx context.Context, id uuid.UUID) (models.ShopMap, error) {
	var shopMap models.ShopMap
	return shopMap, r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&shopMap)
}

func (r *Repo) GetByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]models.ShopMap, error) {
	var shopMaps []models.ShopMap
	cur, err := r.col.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return shopMaps, err
	}

	return shopMaps, cur.All(ctx, &shopMaps)
}

func (r *Repo) Create(ctx context.Context, shopMap models.ShopMap) error {
	_, err := r.col.InsertOne(ctx, shopMap)
	return err
}

func (r *Repo) Update(ctx context.Context, shopMap models.ShopMap) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": shopMap.ID}, shopMap)
	return err
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *Repo) GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.ShopMap, error) {
	var shopMaps []models.ShopMap
	cur, err := r.col.Find(
		ctx,
		bson.M{"owner_id": userID, "viewers_id": bson.M{"$in": userID}},
	)
	if err != nil {
		return shopMaps, err
	}

	return shopMaps, cur.All(ctx, &shopMaps)
}
