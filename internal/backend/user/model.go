package user

import "go-backend/pkg/id"

type Login string

type Hash []byte

type User struct {
	ID id.ID[User]	
	Login Login
	PasswordHash Hash
}

