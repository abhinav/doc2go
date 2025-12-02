// Package testpkg demonstrates single-module versioned linking.
//
// This package exports types and functions that reference third-party packages
// to test that doc2go generates versioned links correctly.
package testpkg

import (
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// Logger is an alias for zap.Logger to create a reference
// to an external package.
//
// Links to [zap.Logger] should be versioned.
type Logger = zap.Logger

// NewLogger creates a new logger.
// It references [zap.NewProduction] which should generate
// a versioned link to go.uber.org/zap@v1.27.1.
func NewLogger() (*zap.Logger, error) {
	return zap.NewProduction()
}

// TestHelper wraps assert.Assertions for testing.
// References to [assert.Assertions] should link to
// github.com/stretchr/testify@v1.8.4/assert.
type TestHelper struct {
	*assert.Assertions
}

// SugaredLogger wraps a SugaredLogger.
// This tests linking to a type from a subpackage.
// The link should be to go.uber.org/zap@v1.27.1#SugaredLogger.
type SugaredLogger = zap.SugaredLogger

// AtomicLevel is an alias to test subpackage references.
// Links should go to go.uber.org/zap@v1.27.1/zapcore#AtomicLevel.
type AtomicLevel = zap.AtomicLevel
