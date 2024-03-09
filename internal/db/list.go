package db

import "go.mongodb.org/mongo-driver/mongo"

type Service struct {
	col *mongo.Collection
}
