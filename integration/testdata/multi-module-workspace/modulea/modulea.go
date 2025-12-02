// Package modulea is the first module in a multi-module workspace.
//
// This module uses zap v1.27.1 and golang.org/x/sync v0.10.0.
// Links from this module should use these versions.
package modulea

import (
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Logger wraps a zap logger.
// Links to [zap.Logger] from this module should point to
// go.uber.org/zap@v1.27.1.
type Logger struct {
	*zap.Logger
}

// ErrGroup wraps an errgroup.
// Links to [errgroup.Group] should point to
// golang.org/x/sync@v0.10.0/errgroup.
type ErrGroup struct {
	*errgroup.Group
}

// NewLogger creates a logger using [zap.NewDevelopment].
func NewLogger() (*Logger, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	return &Logger{Logger: logger}, nil
}
