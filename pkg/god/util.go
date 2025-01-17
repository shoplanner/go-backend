package god

func Believe[T, E any](val T, _ E) T {
	return val
}

func OnlySecond[T, E any](_ T, second E) E {
	return second
}
