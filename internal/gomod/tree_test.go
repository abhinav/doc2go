package gomod_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/gomod"
)

func TestTree_LookupModuleDep(t *testing.T) {
	t.Parallel()

	var tree gomod.Tree

	// Add module "example.com/myproject" with dependencies.
	tree.PutModuleDeps("example.com/myproject", []*gomod.Module{
		{Path: "go.uber.org/zap", Version: "v1.27.1"},
		{Path: "github.com/stretchr/testify", Version: "v1.8.4"},
	})

	// Add another module "example.com/other" with different dependencies.
	tree.PutModuleDeps("example.com/other", []*gomod.Module{
		{Path: "go.uber.org/zap", Version: "v1.26.0"}, // Different version!
		{Path: "github.com/pkg/errors", Version: "v0.9.1"},
	})

	tests := []struct {
		name      string
		source    string
		target    string
		wantInfo  *gomod.Module
		wantFound bool
	}{
		{
			name:      "exact module match",
			source:    "example.com/myproject",
			target:    "go.uber.org/zap",
			wantInfo:  &gomod.Module{Path: "go.uber.org/zap", Version: "v1.27.1"},
			wantFound: true,
		},
		{
			name:      "source subpackage finds module",
			source:    "example.com/myproject/foo/bar",
			target:    "go.uber.org/zap",
			wantInfo:  &gomod.Module{Path: "go.uber.org/zap", Version: "v1.27.1"},
			wantFound: true,
		},
		{
			name:      "target subpackage finds module",
			source:    "example.com/myproject",
			target:    "go.uber.org/zap/zaptest/observer",
			wantInfo:  &gomod.Module{Path: "go.uber.org/zap", Version: "v1.27.1"},
			wantFound: true,
		},
		{
			name:      "both source and target are subpackages",
			source:    "example.com/myproject/internal/auth",
			target:    "github.com/stretchr/testify/assert",
			wantInfo:  &gomod.Module{Path: "github.com/stretchr/testify", Version: "v1.8.4"},
			wantFound: true,
		},
		{
			name:      "different module has different version",
			source:    "example.com/other/pkg",
			target:    "go.uber.org/zap",
			wantInfo:  &gomod.Module{Path: "go.uber.org/zap", Version: "v1.26.0"},
			wantFound: true,
		},
		{
			name:      "unknown dependency",
			source:    "example.com/myproject",
			target:    "golang.org/x/text",
			wantFound: false,
		},
		{
			name:      "unknown source module",
			source:    "unknown.com/project",
			target:    "go.uber.org/zap",
			wantFound: false,
		},
		{
			name:      "stdlib package",
			source:    "example.com/myproject",
			target:    "fmt",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			info := tree.LookupModuleDep(tt.source, tt.target)
			if tt.wantFound {
				require.NotNil(t, info, "expected to find module info")
				assert.Equal(t, tt.wantInfo.Path, info.Path)
				assert.Equal(t, tt.wantInfo.Version, info.Version)
			} else {
				assert.Nil(t, info, "expected no module info")
			}
		})
	}
}

func TestTree_AddModule_EmptyPath(t *testing.T) {
	t.Parallel()

	var tree gomod.Tree
	tree.PutModuleDeps("", []*gomod.Module{
		{Path: "go.uber.org/zap", Version: "v1.27.1"},
	})

	// Should not panic, and lookup should return nil.
	info := tree.LookupModuleDep("example.com/myproject", "go.uber.org/zap")
	assert.Nil(t, info)
}

func TestTree_AddModule_NilDeps(t *testing.T) {
	t.Parallel()

	var tree gomod.Tree
	tree.PutModuleDeps("example.com/myproject", []*gomod.Module{
		nil,
		{Path: "", Version: "v1.0.0"},
		{Path: "go.uber.org/zap", Version: "v1.27.1"},
	})

	// Should handle nil and empty path gracefully.
	info := tree.LookupModuleDep("example.com/myproject", "go.uber.org/zap")
	require.NotNil(t, info)
	assert.Equal(t, "v1.27.1", info.Version)
}
