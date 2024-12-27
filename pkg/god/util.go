package god

func Believe[T any](val T, _ error) T {
	return val
}
