package list

import (
	"context"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Repo struct {
	col *mongo.Collection
}

func (r *Repo) ID(ctx context.Context, id uuid.UUID) (ProductListResponse, error) {
	var productList ProductListResponse

	res := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}})
	if res.Err() != nil {
		return productList, res.Err()
	}
	if err := res.Decode(&productList); err != nil {
		return productList, err
	}
	return productList, nil
}

func (r *Repo) Create(ctx context.Context, request ProductListResponse) error {
	_, err := r.col.InsertOne(ctx, request)
	return err
}

func (r *Repo) UserID(ctx context.Context, userID uuid.UUID) ([]ProductListResponse, error) {
	var lists []ProductListResponse

	res, err := r.col.Find(ctx, bson.D{{Key: "user_id", Value: userID}})
	if err != nil {
		return nil, err
	} else if res.Err() != nil {
		return nil, err
	}

	return lists, res.Decode(&lists)
}

func (r *Repo) Update(ctx context.Context, request ProductListResponse) (ProductListResponse, error) {
	res := r.col.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: request.ID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "updated_at", Value: request.UpdatedAt},
			{Key: "name", Value: request.Name},
			{Key: "states", Value: request.States},
			{Key: "status", Value: request.Status},
			{Key: "view_id_list", Value: request.ViewerIDList},
		}}})
	if res.Err() != nil {
		return request, res.Err()
	}
	return request, res.Decode(&request)
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) (ProductListResponse, error) {
	var list ProductListResponse

	res := r.col.FindOneAndDelete(ctx, bson.D{{Key: "_id", Value: id}})
	if res.Err() != nil {
		return list, res.Err()
	}

	return list, res.Decode(&list)
}
