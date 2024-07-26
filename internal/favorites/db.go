package favorites

import (
	"context"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
)

type Repo struct {
	col *mongo.Collection
}

func NewRepo(db *mongo.Database) *Repo {
    return &Repo{
        col: db.Collection("favorites"),
    }
}

func (r *Repo) ID(ctx context.Context, ) error {

}

func (r *Repo) UserID(ctx context.Context, userID uuid.UUID) (List, error) P
var list 
