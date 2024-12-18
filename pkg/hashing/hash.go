package hashing

import (
	"fmt"
	"unsafe"

	"golang.org/x/crypto/bcrypt"
)

type HashMaster struct{}

func (HashMaster) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(strToBytes(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("can't hash password: %w", err)
	}

	return bytesToStr(hash), nil
}

func (HashMaster) Compare(password string, hash string) bool {
	return bcrypt.CompareHashAndPassword(strToBytes(hash), strToBytes(password)) == nil
}

func strToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func bytesToStr(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
