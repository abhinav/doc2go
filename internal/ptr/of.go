package ptr

// Of returns a pointer to a value of the given type.
// This is a convenience function to turn literals into pointers.
func Of[T any](v T) *T {
	return &v
}
