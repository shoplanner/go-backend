package repo

import (
	"context"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Repo struct {
	col *mongo.Collection
}

func NewRepo(collection *mongo.Collection) *Repo {
	return &Repo{col: collection}
}

func (r *Repo) ID(ctx context.Context, id uuid.UUID) (Response, error) {
	var product Response

	res := r.col.FindOne(ctx, bson.D{{"_id", id}})
	if res.Err() != nil {
		return product, res.Err()
	}
	if err := res.Decode(&product); err != nil {
		return product, err
	}
	return product, nil
}

func (r *Repo) IDList(ctx context.Context, ids []uuid.UUID) ([]Response, error) {
	var list []Response

	res, err := r.col.Find(ctx, bson.D{{"_id", bson.D{{"$in", ids}}}})
	if err != nil {
		return nil, err
	}

	return list, res.All(ctx, &list)
}

func (r *Repo) Create(ctx context.Context, product Response) error {
	_, err := r.col.InsertOne(ctx, product)
	return err
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) (Response, error) {
	var product Response

	res := r.col.FindOneAndDelete(ctx, bson.D{{"_id", id}})
	if res.Err() != nil {
		return product, res.Err()
	}

	return product, res.Decode(&product)
}

func (r *Repo) Update(ctx context.Context, product Response) (Response, error) {
	res := r.col.FindOneAndUpdate(ctx, bson.D{{"_id", product.ID}}, product)
	if res.Err() != nil {
		return product, res.Err()
	}
	return product, res.Decode(&product)
}
