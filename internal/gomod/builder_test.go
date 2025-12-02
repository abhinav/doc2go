package gomod

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/gosrc"
)

func TestBuilder_Build(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Module 1: example.com/foo
	fooDir := filepath.Join(tmpDir, "foo")
	err := os.MkdirAll(fooDir, 0o755)
	require.NoError(t, err)

	fooGoMod := `module example.com/foo

require (
	go.uber.org/zap v1.27.1
	github.com/stretchr/testify v1.8.4
)
`
	err = os.WriteFile(filepath.Join(fooDir, "go.mod"), []byte(fooGoMod), 0o644)
	require.NoError(t, err)

	fooGo := filepath.Join(fooDir, "foo.go")
	err = os.WriteFile(fooGo, []byte("package foo"), 0o644)
	require.NoError(t, err)

	// Module 2: example.com/bar
	barDir := filepath.Join(tmpDir, "bar")
	err = os.MkdirAll(barDir, 0o755)
	require.NoError(t, err)

	barGoMod := `module example.com/bar

require (
	go.uber.org/zap v1.26.0
	golang.org/x/text v0.14.0
)
`
	err = os.WriteFile(filepath.Join(barDir, "go.mod"), []byte(barGoMod), 0o644)
	require.NoError(t, err)

	barGo := filepath.Join(barDir, "bar.go")
	err = os.WriteFile(barGo, []byte("package bar"), 0o644)
	require.NoError(t, err)

	// Create PackageRefs.
	pkgs := []*gosrc.PackageRef{
		{
			ImportPath: "example.com/foo",
			Module: &gosrc.ModuleRef{
				Path:  "example.com/foo",
				GoMod: filepath.Join(fooDir, "go.mod"),
			},
			Files: []string{fooGo},
		},
		{
			ImportPath: "example.com/bar",
			Module: &gosrc.ModuleRef{
				Path:  "example.com/bar",
				GoMod: filepath.Join(barDir, "go.mod"),
			},
			Files: []string{barGo},
		},
	}

	builder := &Builder{
		Logger: log.New(t.Output(), "", log.LstdFlags),
	}
	tree := builder.Build(pkgs)

	require.NotNil(t, tree)

	// Test lookups from foo module.
	info := tree.LookupModuleDep("example.com/foo", "go.uber.org/zap")
	require.NotNil(t, info)
	assert.Equal(t, "go.uber.org/zap", info.Path)
	assert.Equal(t, "v1.27.1", info.Version)

	info = tree.LookupModuleDep("example.com/foo/subpkg", "github.com/stretchr/testify/assert")
	require.NotNil(t, info)
	assert.Equal(t, "github.com/stretchr/testify", info.Path)
	assert.Equal(t, "v1.8.4", info.Version)

	// Test lookups from bar module (different version of zap).
	info = tree.LookupModuleDep("example.com/bar", "go.uber.org/zap")
	require.NotNil(t, info)
	assert.Equal(t, "go.uber.org/zap", info.Path)
	assert.Equal(t, "v1.26.0", info.Version)

	info = tree.LookupModuleDep("example.com/bar", "golang.org/x/text")
	require.NotNil(t, info)
	assert.Equal(t, "golang.org/x/text", info.Path)
	assert.Equal(t, "v0.14.0", info.Version)

	// Test unknown dependency.
	info = tree.LookupModuleDep("example.com/foo", "github.com/pkg/errors")
	assert.Nil(t, info)
}

func TestBuilder_Build_noModules(t *testing.T) {
	t.Parallel()

	// GOPATH packages with no module.
	pkgs := []*gosrc.PackageRef{
		{
			ImportPath: "example.com/foo",
			Module:     nil, // No module
		},
	}

	builder := &Builder{
		Logger: log.New(t.Output(), "", log.LstdFlags),
	}
	tree := builder.Build(pkgs)

	// Should return empty tree for no modules.
	assert.NotNil(t, tree)
	assert.Nil(t, tree.LookupModuleDep("example.com/foo", "anything"))
}

func TestBuilder_Build_missingGoMod(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fooGo := filepath.Join(tmpDir, "foo.go")
	err := os.WriteFile(fooGo, []byte("package foo"), 0o644)
	require.NoError(t, err)

	pkgs := []*gosrc.PackageRef{
		{
			ImportPath: "example.com/foo",
			Module: &gosrc.ModuleRef{
				Path:  "example.com/foo",
				GoMod: filepath.Join(tmpDir, "go.mod"), // Doesn't exist
			},
			Files: []string{fooGo},
		},
	}

	// Create a logger that writes to a buffer so we can check the warning.
	var logBuf bytes.Buffer
	builder := &Builder{
		Logger: log.New(&logBuf, "", 0),
	}
	tree := builder.Build(pkgs)

	// Should still create tree, but no dependencies.
	// Tree will be empty but not nil.
	require.NotNil(t, tree)

	// No dependencies should be found.
	info := tree.LookupModuleDep("example.com/foo", "go.uber.org/zap")
	assert.Nil(t, info)

	// Should have logged a warning about the missing go.mod.
	assert.Contains(t, logBuf.String(), "error parsing go.mod")
}

func TestParseGomod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		goModContent string
		modulePath   string
		wantDeps     map[string]string // path -> version
	}{
		{
			name: "simple module with dependencies",
			goModContent: `module example.com/myproject

go 1.21

require (
	go.uber.org/zap v1.27.1
	github.com/stretchr/testify v1.8.4
)
`,
			modulePath: "example.com/myproject",
			wantDeps: map[string]string{
				"go.uber.org/zap":             "v1.27.1",
				"github.com/stretchr/testify": "v1.8.4",
			},
		},
		{
			name: "module with replace directive",
			goModContent: `module example.com/myproject

require (
	go.uber.org/zap v1.27.1
	github.com/pkg/errors v0.9.1
)

replace go.uber.org/zap => go.uber.org/zap v1.26.0
`,
			modulePath: "example.com/myproject",
			wantDeps: map[string]string{
				"go.uber.org/zap":       "v1.26.0", // Replaced version
				"github.com/pkg/errors": "v0.9.1",
			},
		},
		{
			name: "module with indirect dependencies",
			goModContent: `module example.com/myproject

require (
	go.uber.org/zap v1.27.1
	golang.org/x/text v0.14.0 // indirect
)
`,
			modulePath: "example.com/myproject",
			wantDeps: map[string]string{
				"go.uber.org/zap":   "v1.27.1",
				"golang.org/x/text": "v0.14.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			goModPath := filepath.Join(tmpDir, "go.mod")
			err := os.WriteFile(goModPath, []byte(tt.goModContent), 0o644)
			require.NoError(t, err)

			ref := &gosrc.ModuleRef{
				Path:  tt.modulePath,
				GoMod: goModPath,
			}
			deps, err := parseGomod(ref)
			require.NoError(t, err)

			require.NotNil(t, deps)
			assert.Len(t, deps, len(tt.wantDeps))

			gotDeps := make(map[string]string)
			for _, dep := range deps {
				gotDeps[dep.Path] = dep.Version
			}
			assert.Equal(t, tt.wantDeps, gotDeps)
		})
	}
}

func TestParseGomod_invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		goModContent string
	}{
		{
			name:         "invalid syntax",
			goModContent: `this is not valid go.mod syntax`,
		},
		{
			name: "no module directive",
			goModContent: `go 1.21

require go.uber.org/zap v1.27.1
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			goModPath := filepath.Join(tmpDir, "go.mod")
			err := os.WriteFile(goModPath, []byte(tt.goModContent), 0o644)
			require.NoError(t, err)

			ref := &gosrc.ModuleRef{
				Path:  "example.com/test",
				GoMod: goModPath,
			}
			_, err = parseGomod(ref)
			assert.Error(t, err)
		})
	}
}

func TestParseGomod_missingFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ref := &gosrc.ModuleRef{
		Path:  "example.com/test",
		GoMod: filepath.Join(tmpDir, "nonexistent.mod"),
	}
	_, err := parseGomod(ref)

	assert.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
