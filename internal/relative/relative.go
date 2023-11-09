// Package relative turns paths and file paths relative
// with string manipulation exclusively.
package relative

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"go.abhg.dev/doc2go/internal/sliceutil"
)

const (
	_slash       = "/"
	_filepathSep = string(filepath.Separator)
)

// Path returns a path to dst, relative to src.
// Both paths must be relative or both paths must be absolute,
// and they must both be /-separated.
//
// This operation relies on string manipulation exlusively,
// so it doesn't fail.
func Path(src, dst string) string {
	return rel(_slash, src, dst)
}

// Filepath returns a path to dst, relative to src.
// Both paths must be relative or both paths must be absolute,
// and they must both be valid file paths for the current system.
//
// This operation relies on string manipulation exlusively,
// so it doesn't fail.
func Filepath(src, dst string) string {
	return rel(_filepathSep, src, dst)
}

func rel(delim, src, dst string) string {
	if path.IsAbs(src) != path.IsAbs(dst) {
		panic(fmt.Sprintf("Rel(%q, %q): both must be absolute, or both must be relative", src, dst))
	}
	// src must always be a directory.
	// Drop the trailing /, if any.
	src = strings.TrimSuffix(src, delim)

	var srcParts, dstParts []string
	if len(src) > 0 {
		srcParts = strings.Split(src, delim)
	}
	if len(dst) > 0 {
		dstParts = strings.Split(dst, delim)
	}

	srcParts, dstParts = sliceutil.RemoveCommonPrefix(srcParts, dstParts)

	var sb strings.Builder
	for range srcParts {
		if sb.Len() > 0 {
			sb.WriteString(delim)
		}
		sb.WriteString("..")
	}
	for _, p := range dstParts {
		if sb.Len() > 0 {
			sb.WriteString(delim)
		}
		sb.WriteString(p)
	}

	return sb.String()
}
