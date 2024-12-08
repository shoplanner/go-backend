package ph

func EmptyStruct[T any](t T) (T, struct{}) {
	return t, struct{}{}
}
