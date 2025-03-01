package hashing_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"go-backend/pkg/hashing"
)

func TestHash(t *testing.T) {
	suite.Run(t, new(HashSuite))
}

type HashSuite struct {
	suite.Suite

	hash hashing.HashMaster
}

func (s *HashSuite) TestCompare() {
	s.Run("hash can be compared", func() {
		const password = "pas$w0rD"

		hash, err := s.hash.HashPassword(password)
		s.Require().NoError(err)

		s.True(s.hash.Compare(password, hash))
	})
	s.Run("wrong password", func() {
		const password = "pas$w0rD"

		hash, err := s.hash.HashPassword(password)
		s.Require().NoError(err)

		s.False(s.hash.Compare(password+"kek", hash))
	})
}
