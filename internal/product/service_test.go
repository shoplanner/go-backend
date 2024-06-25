package product_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestProductService(t *testing.T) {
	suite.Run(t, new(ProductServiceSuite))
}

func (s *ProductServiceSuite) TestKEK() {
	cases := []struct {
		name string
		err  error
	}{
		{
			name: "kek",
			err:  assert.AnError,
		},
		{
			name: "lol",
		},
	}

	for _, test := range cases {
		s.Run(test.name, func() {
			if test.err != nil {
				s.Fail(test.err.Error())
			}
		})
	}
}

func TestSmth(t *testing.T) {
	t.Run("noname", func(t *testing.T) {
		t.Fail()
	})

	t.Run("newname", func(t *testing.T) {
	})


}

func (s *ProductServiceSuite) TestLOL() {
	s.Run("first", func() {
		const a = 4
		if 1 != 2 || a == 4 {
			s.Fail("oh no")
		}
	})

	s.Run("kek", func() {
		if 2 == 3 {
			s.Fail("yes")
		}
	})
}

type ProductServiceSuite struct {
	suite.Suite
}
