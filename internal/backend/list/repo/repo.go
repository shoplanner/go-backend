package repo

import (
	"context"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"go-backend/internal/list/models"
)

type Repo struct {
	col *mongo.Collection
}

func (r *Repo) ID(ctx context.Context, id uuid.UUID) (models.ProductList, error) {
	var productList models.ProductList

	res := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}})
	if res.Err() != nil {
		return productList, res.Err()
	}
	if err := res.Decode(&productList); err != nil {
		return productList, err
	}
	return productList, nil
}

func (r *Repo) Create(ctx context.Context, request models.ProductList) error {
	_, err := r.col.InsertOne(ctx, request)
	return err
}

func (r *Repo) UserID(ctx context.Context, userID uuid.UUID) ([]models.ProductList, error) {
	var lists []models.ProductList

	res, err := r.col.Find(ctx, bson.D{{Key: "user_id", Value: userID}})
	if err != nil {
		return nil, err
	} else if res.Err() != nil {
		return nil, err
	}

	return lists, res.Decode(&lists)
}

func (r *Repo) Update(ctx context.Context, productList models.ProductList) (models.ProductList, error) {
	res := r.col.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: productList.ID}}, productList)
	if res.Err() != nil {
		return productList, res.Err()
	}
	return productList, res.Decode(&productList)
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) (models.ProductList, error) {
	var list models.ProductList

	res := r.col.FindOneAndDelete(ctx, bson.D{{Key: "_id", Value: id}})
	if res.Err() != nil {
		return list, res.Err()
	}

	return list, res.Decode(&list)
}
