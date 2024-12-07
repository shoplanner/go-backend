package iterators

import "iter"

func Map[T, E any](mapFunc func(T) E) iter.Seq[T] {
	return func(yield func(T) bool) {
	}
}
