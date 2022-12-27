package slices

// RemoveFunc removes items matching the given function
// from the provided slice.
//
// The original slice must not be used after this.
func RemoveFunc[T any](items []T, skip func(T) bool) []T {
	newItems := items[:0]
	for _, item := range items {
		if !skip(item) {
			newItems = append(newItems, item)
		}
	}
	return newItems
}
