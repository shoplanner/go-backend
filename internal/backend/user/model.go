package user

import "go-backend/pkg/id"

//go:generate go-enum --marshal --names --values

// ENUM(admin=1, user)
type Role int

type Login string

type Hash []byte

type User struct {
	ID           id.ID[User]
	Role         Role
	Login        Login
	PasswordHash Hash
}
