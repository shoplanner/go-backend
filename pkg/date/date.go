package date

import "time"

type CreateDate[T any] struct {
	time.Time `bson:"inline"`
}

type UpdateDate[T any] struct {
	time.Time `bson:"inline"`
}
