package repo

import (
	"context"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"go-backend/internal/product/models"
)

type Repo struct {
	col *mongo.Collection
}

func NewRepo(collection *mongo.Collection) *Repo {
	return &Repo{col: collection}
}

func (r *Repo) ID(ctx context.Context, id uuid.UUID) (models.Product, error) {
	var product models.Product

	res := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}})
	if res.Err() != nil {
		return product, res.Err()
	}
	if err := res.Decode(&product); err != nil {
		return product, err
	}
	return product, nil
}

func (r *Repo) IDList(ctx context.Context, ids []uuid.UUID) ([]models.Product, error) {
	var list []models.Product

	res, err := r.col.Find(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}})
	if err != nil {
		return nil, err
	}

	return list, res.All(ctx, &list)
}

func (r *Repo) Create(ctx context.Context, product models.Product) error {
	_, err := r.col.InsertOne(ctx, product)
	return err
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) (models.Product, error) {
	var product models.Product

	res := r.col.FindOneAndDelete(ctx, bson.D{{Key: "_id", Value: id}})
	if res.Err() != nil {
		return product, res.Err()
	}

	return product, res.Decode(&product)
}

func (r *Repo) Update(ctx context.Context, product models.Product) (models.Product, error) {
	res := r.col.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: product.ID}}, product)
	if res.Err() != nil {
		return product, res.Err()
	}
	return product, res.Decode(&product)
}

func (r *Repo) IsExist(ctx context.Context, id uuid.UUID) (bool, error) {
	res, err := r.col.CountDocuments(ctx, bson.D{{Key: "_id", Value: id}})
	return res > 0, err
}
