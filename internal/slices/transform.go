package slices

// Transform builds a slice by applying the provided function
// to all elements in the given slice.
func Transform[From, To any](from []From, f func(From) To) []To {
	if len(from) == 0 {
		return nil
	}
	to := make([]To, len(from))
	for i, v := range from {
		to[i] = f(v)
	}
	return to
}
