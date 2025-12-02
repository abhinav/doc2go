// Package moduleb is the second module in a multi-module workspace.
//
// This module uses DIFFERENT versions:
// - zap v1.26.0 (different from modulea's v1.27.1)
// - golang.org/x/sync v0.9.0 (different from modulea's v0.10.0)
//
// This tests that doc2go correctly generates different versioned links
// for the same package based on which module is doing the linking.
package moduleb

import (
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Logger wraps a zap logger.
// Links to [zap.Logger] from THIS module should point to
// go.uber.org/zap@v1.26.0 (NOT v1.27.1 like modulea).
type Logger struct {
	*zap.Logger
}

// ErrGroup wraps an errgroup.
// Links to [errgroup.Group] from THIS module should point to
// golang.org/x/sync@v0.9.0/errgroup (NOT v0.10.0 like modulea).
type ErrGroup struct {
	*errgroup.Group
}

// NewProduction creates a production logger using [zap.NewProduction].
// The link should be to go.uber.org/zap@v1.26.0.
func NewProduction() (*Logger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return &Logger{Logger: logger}, nil
}
