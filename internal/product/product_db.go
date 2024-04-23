package product

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

func (r *Repo) ID(ctx context.Context, id uuid.UUID) (ProductResponse, error) {
	var product ProductResponse

	res := r.col.FindOne(ctx, bson.D{{"_id", id}})
	if res.Err() != nil {
		return product, res.Err()
	}
	if err := res.Decode(&product); err != nil {
		return product, err
	}
	return product, nil
}

func (r *Repo) IDList(ctx context.Context, ids []uuid.UUID) ([]ProductResponse, error) {
	var list []ProductResponse

	res, err := r.col.Find(ctx, bson.D{{"_id", bson.D{{"$in", ids}}}})
	if err != nil {
		return nil, err
	}

	return list, res.All(ctx, &list)
}

func (r *Repo) Create(ctx context.Context, product ProductResponse) error {
	_, err := r.col.InsertOne(ctx, product)
	return err
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) (ProductResponse, error) {
	var product ProductResponse

	res := r.col.FindOneAndDelete(ctx, bson.D{{"_id", id}})
	if res.Err() != nil {
		return product, res.Err()
	}

	return product, res.Decode(&product)
}

func (r *Repo) Update(ctx context.Context, product ProductResponse) (ProductResponse, error) {
	res := r.col.FindOneAndUpdate(ctx, bson.D{{"_id", product.ID}},
		bson.D{{"$set", bson.D{
			{"name", product.Name},
			{"category", product.Category},
			{"forms", product.Forms},
			{"updated_at", product.UpdatedAt},
		}}})
	if res.Err() != nil {
		return product, res.Err()
	}
	return product, res.Decode(&product)
}
