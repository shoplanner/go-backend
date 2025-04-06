package deepcopy

import "github.com/mohae/deepcopy"

func MustCopy[T any](toCopy T) T {
	return deepcopy.Copy(toCopy).(T)
}
