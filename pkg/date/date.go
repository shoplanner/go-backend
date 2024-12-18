package date

import "time"

type CreateDate[T any] struct {
	time.Time
}

func NewCreateDate[T any]() CreateDate[T] {
	return CreateDate[T]{Time: time.Now()}
}

type UpdateDate[T any] struct {
	time.Time
}

func NewUpdateDate[T any]() UpdateDate[T] {
	return UpdateDate[T]{Time: time.Now()}
}

func (d *UpdateDate[T]) Update() {
	d.Time = time.Now()
}
