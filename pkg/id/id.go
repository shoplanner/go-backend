package id

import "github.com/google/uuid"

type ID[T any] struct {
	uuid.UUID
}

func NewID[T any]() ID[T] {
	return ID[T]{ UUID: uuid.New(), }
}