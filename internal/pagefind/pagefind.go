// Package pagefind provides access to the pagefind CLI.
package pagefind

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"

	"braces.dev/errtrace"
	"go.abhg.dev/doc2go/internal/linebuf"
)

// CLI is a handle to the pagefind CLI,
// which is used to generate a search index for the documentation.
type CLI struct {
	// Pagefind is the path to the pagefind executable.
	// If unset, we'll search $PATH.
	Pagefind string

	// Log is the logger to use for the output of the pagefind command.
	Log *log.Logger
}

// IndexRequest is a request to generate a search index
// for a website.
type IndexRequest struct {
	// SiteDir is the path to the static website to index.
	SiteDir string // required

	// Path to the directory where pagefind assets are stored
	// relative to SiteDir.
	AssetSubdir string
}

// Index generates a search index for a provided website.
func (c *CLI) Index(ctx context.Context, req IndexRequest) error {
	logger := c.Log
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	exe := c.Pagefind
	if exe == "" {
		exe = "pagefind"
	}

	args := []string{
		"--site", req.SiteDir, "--verbose",
	}
	if req.AssetSubdir != "" {
		args = append(args, "--output-subdir", req.AssetSubdir)
	}

	out, done := linebuf.Writer(func(line []byte) {
		logger.Printf("%s", bytes.TrimSuffix(line, []byte{'\n'}))
	})
	defer done()

	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return errtrace.Wrap(fmt.Errorf("pagefind: %w", err))
	}

	return nil
}
