// Package replacepkg tests replace directive handling.
//
// This package has a go.mod with replace directives:
// 1. Version replacement: zap v1.27.1 => v1.26.0
// 2. Local path replacement: pkg/errors => ./vendor/errors
// 3. No replacement: golang.org/x/text uses required version
package replacepkg

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/text/language"
)

// Logger wraps a zap logger.
// Even though go.mod requires v1.27.1, the replace directive
// should cause links to point to go.uber.org/zap@v1.26.0.
type Logger struct {
	*zap.Logger
}

// Error wraps pkg/errors.
// Since this has a local path replacement,
// it may not have version information.
// Links should fall back to unversioned pkg.go.dev.
type Error interface {
	error
	Cause() error
}

// NewError creates an error using [errors.New].
// This tests linking to a package with local path replacement.
func NewError(msg string) error {
	return errors.New(msg)
}

// Tag wraps a language tag.
// This package has NO replace directive,
// so links should use the version from require:
// golang.org/x/text@v0.14.0/language.
type Tag = language.Tag

// ParseTag parses a language tag using [language.Parse].
func ParseTag(s string) (language.Tag, error) {
	return language.Parse(s)
}

// Config wraps zap config.
// Tests linking to a type from subpackage with version replacement.
// Should link to go.uber.org/zap@v1.26.0/zapcore.
type Config = zap.Config
