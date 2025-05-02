package deepcopy

import (
	"fmt"

	"github.com/jinzhu/copier"
)

func MustCopy[T any](toCopy T) T {
	var res T
	err := copier.Copy(&res, toCopy)
	if err != nil {
		panic(fmt.Sprintf("failed to copy: %s", err.Error()))
	}

	return res
}
