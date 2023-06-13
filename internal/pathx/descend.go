// Package pathx provides extensions to the [path] package.
package pathx

import "strings"

// Descends reports whether b is equal to, or a descendant of a.
func Descends(a, b string) bool {
	a = strings.TrimSuffix(a, "/")
	if !strings.HasPrefix(b, a) {
		return false
	}
	b = b[len(a):]
	return b == "" || b[0] == '/'
}
