package sliceutil

// RemoveCommonPrefix removes the shared prefix from the two provided slices,
// returning what remains for each slice as the result.
func RemoveCommonPrefix[T comparable](a, b []T) (newA, newB []T) {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i:], b[i:]
		}
	}
	switch na, nb := len(a), len(b); {
	case na < nb:
		return nil, b[na:]
	case na > nb:
		return a[nb:], nil
	default:
		return nil, nil
	}
}
