//go:build tools
// +build tools

package tools

// Tools we use during development.
import (
	_ "golang.org/x/lint/golint"
	_ "honnef.co/go/tools/cmd/staticcheck"
)
