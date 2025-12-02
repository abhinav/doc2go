package integration

import (
	"flag"
	"io"
	"iter"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/cascadia"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/container/ring"
	"go.abhg.dev/doc2go/internal/iotest"
	"go.abhg.dev/doc2go/internal/pathx"
	"golang.org/x/net/html"
)

var (
	_doc2go   = flag.String("doc2go", "", "path to doc2go binary")
	_pagefind = flag.String("pagefind", "", "path to pagefind binary")
	_rundir   = flag.String("rundir", "", "path to directory to run doc2go in")
)

func TestMain(m *testing.M) {
	flag.Parse()

	if *_doc2go == "" {
		var err error
		*_doc2go, err = exec.LookPath("doc2go")
		if err != nil {
			log.Fatal("doc2go not found in PATH: ", err)
		}
	}

	os.Exit(m.Run())
}

func TestLinksAreValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		basename string // index file basename if using rel-link-style=index
	}{
		{name: "self", args: []string{"./..."}},
		{name: "exact home", args: []string{"-home=go.abhg.dev/doc2go", "./..."}},
		{name: "parent home", args: []string{"-home=go.abhg.dev", "./..."}},
		{
			name: "child home",
			args: []string{
				"-home", "github.com/stretchr/testify/assert",
				"github.com/stretchr/testify/...",
			},
		},
		{
			name: "rel-link-style=directory",
			args: []string{"-rel-link-style=directory", "./..."},
		},
		{
			name:     "rel-link-style=index",
			args:     []string{"-rel-link-style=index", "./..."},
			basename: "index.html",
		},
		{
			name:     "rel-link-style=index with custom basename",
			args:     []string{"-rel-link-style=index", "-basename=_index.html", "./..."},
			basename: "_index.html",
		},
		{
			name: "home with subdir",
			args: []string{
				"-home", "go.abhg.dev/doc2go",
				"-subdir", "v1.2.3",
				"./...",
			},
		},
		{
			name: "pagefind",
			args: []string{"-pagefind=" + *_pagefind, "./..."},
		},
		{
			name: "pagefind with home",
			args: []string{"-home=go.abhg.dev/doc2go", "-pagefind=" + *_pagefind, "./..."},
		},
		{
			name: "pagefind with subdir",
			args: []string{"-subdir=v1.2.3", "-pagefind=" + *_pagefind, "./..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := generate(t, tt.args...)
			visitLocalURLs(t, dir, &visitOptions{Basename: tt.basename})
		})
	}
}

func TestDocumentationIsRelocatable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "self", args: []string{"./..."}},
		{name: "parent home", args: []string{"-home=go.abhg.dev", "./..."}},
		{
			name: "home with subdir",
			args: []string{
				"-home", "go.abhg.dev/doc2go",
				"-subdir", "v1.2.3",
				"./...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Put the documentation in a subdirectory of an HTTP server.
			// None of the links should hit the server root.
			root := t.TempDir()

			dir := filepath.Join(root, "api")
			args := append([]string{"-out=" + dir}, tt.args...)
			generate(t, args...)

			visitLocalURLs(t, root, &visitOptions{
				StartPage: "/api/",
				ShouldVisit: func(local localURL) bool {
					if local.Kind != localPage {
						return false
					}

					// All pages must have a "/api/" prefix.
					return assert.True(t,
						pathx.Descends("/api/", local.URL.Path),
						"link %v breaks out of /api/", local.URL)
				},
			})
		})
	}
}

// https://github.com/abhinav/doc2go/issues/176
func TestNoInternalPackagesListed(t *testing.T) {
	t.Parallel()

	dir := generate(t, "-internal=false", "go/...")
	// index.html should not contain "go/internal/*".
	index := filepath.Join(dir, "index.html")
	b, err := os.ReadFile(index)
	require.NoError(t, err)

	assert.NotContains(t, string(b), "go/internal/")
}

// Verifies that with -rel-link-style=directory,
// all relative links in generated HTML
// have a '/' suffix.
func TestDirectoryRelativeLinks(t *testing.T) {
	t.Parallel()

	root := generate(t, "-rel-link-style=directory", "./...")
	visitLocalURLs(t, root, &visitOptions{ShouldVisit: func(local localURL) bool {
		if local.Kind == localAsset {
			return assert.False(t,
				strings.HasSuffix(local.URL.Path, "/"),
				"%v: path for relative asset ends with '/': %v", local.From, local.Href)
		}

		href := local.Href
		u, err := url.Parse(href)
		require.NoError(t, err, "%v: bad URL: %v", local.From, href)
		if u.IsAbs() || len(u.Host) > 0 || len(u.Path) == 0 {
			return true
		}

		return assert.True(t,
			strings.HasSuffix(u.Path, "/"),
			"%v: path for relative link does not end with '/': %v", local.From, href)
	}})
}

// Verifies that with -rel-link-style=index,
// all relative links to PACKAGE PAGES in generated HTML
// end with '/index.html' (or just 'index.html' for self-references).
func TestIndexRelativeLinks(t *testing.T) {
	t.Parallel()

	root := generate(t, "-rel-link-style=index", "./...")
	basename := "index.html"
	visitLocalURLs(t, root, &visitOptions{
		Basename: basename,
		ShouldVisit: func(local localURL) bool {
			if local.Kind == localAsset {
				return assert.False(t,
					strings.HasSuffix(local.URL.Path, "/"+basename),
					"%v: relative asset incorrectly ends with '/%s': %v", local.From, basename, local.Href)
			}

			href := local.Href
			u, err := url.Parse(href)
			require.NoError(t, err, "%v: bad URL: %v", local.From, href)
			if u.IsAbs() || len(u.Host) > 0 || len(u.Path) == 0 {
				return true
			}

			// For relative links to package pages, check they end with the basename
			// or are directory links (ending with "/")
			if strings.HasSuffix(u.Path, "/") && !strings.HasSuffix(u.Path, "/"+basename) {
				// Directory link like "_/" - this is OK
				return true
			}

			return assert.True(t,
				strings.HasSuffix(u.Path, "/"+basename) || u.Path == basename,
				"%v: relative link does not end with '%s': %v", local.From, basename, href)
		},
	})
}

// Verifies that with -rel-link-style=index and a custom basename,
// all relative links to PACKAGE PAGES in generated HTML
// end with the custom basename.
func TestIndexRelativeLinksCustomBasename(t *testing.T) {
	t.Parallel()

	basename := "_index.html"
	root := generate(t, "-rel-link-style=index", "-basename="+basename, "./...")
	visitLocalURLs(t, root, &visitOptions{
		Basename: basename,
		ShouldVisit: func(local localURL) bool {
			if local.Kind == localAsset {
				return assert.False(t,
					strings.HasSuffix(local.URL.Path, "/"+basename),
					"%v: relative asset incorrectly ends with '/%s': %v", local.From, basename, local.Href)
			}

			href := local.Href
			u, err := url.Parse(href)
			require.NoError(t, err, "%v: bad URL: %v", local.From, href)
			if u.IsAbs() || len(u.Host) > 0 || len(u.Path) == 0 {
				return true
			}

			// For relative links to package pages, check they end with the basename
			// or are directory links (ending with "/")
			if strings.HasSuffix(u.Path, "/") && !strings.HasSuffix(u.Path, "/"+basename) {
				// Directory link like "_/" - this is OK
				return true
			}

			return assert.True(t,
				strings.HasSuffix(u.Path, "/"+basename) || u.Path == basename,
				"%v: relative link does not end with '%s': %v", local.From, basename, href)
		},
	})
}

// Verifies that multiple runs with different -subdir
// generate a shared root index page.
func TestOutputSubdir(t *testing.T) {
	t.Parallel()

	outDir := generate(t, "-subdir=v1.1.0", "./...")
	generate(t, "-subdir=v1.2.3", "-out="+outDir, "./...")

	// Verify that we hit /v1.1.0/ and /v1.2.3/
	roots := make(map[string]struct{})
	visitLocalURLs(t, outDir, &visitOptions{ShouldVisit: func(local localURL) bool {
		if local.Kind != localPage {
			return false
		}

		path := strings.TrimPrefix(local.URL.Path, "/")
		if root, _, ok := strings.Cut(path, "/"); ok {
			roots[root] = struct{}{}
		}
		return true
	}})

	assert.Equal(t, map[string]struct{}{
		"v1.1.0": {},
		"v1.2.3": {},
	}, roots)
}

func generate(t *testing.T, args ...string) (outDir string) {
	// This function has a few convenient defaults:
	//
	// - Unless -internal=false is explicitly specified,
	//   we'll always enable internal packages.
	// - Unless an output directory is explicitly specified,
	//   we'll generate to a temporary directory.
	// - Unless a pagefind argument is explicitly specified,
	//   we'll disable pagefind.
	// - We always enable debug logging.

	var noInternal, pagefindArg bool
	for i, arg := range args {
		if v, ok := strings.CutPrefix(arg, "-out="); ok {
			outDir = v
			continue
		}
		if arg == "-out" && i+1 < len(args) {
			outDir = args[i+1]
			continue
		}

		if strings.HasPrefix(arg, "-pagefind=") {
			pagefindArg = true
		} else if arg == "-pagefind" && i+1 < len(args) {
			pagefindArg = true
		}

		if arg == "-internal=false" {
			noInternal = true
		}
	}

	if outDir == "" {
		outDir = t.TempDir()
	}

	extraArgs := []string{"-out=" + outDir, "-debug"}
	if !noInternal {
		extraArgs = append(extraArgs, "-internal")
	}
	if !pagefindArg {
		extraArgs = append(extraArgs, "-pagefind=false")
	}

	output := iotest.Writer(t)
	cmd := exec.Command(*_doc2go, append(extraArgs, args...)...)
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.Dir = *_rundir
	require.NoError(t, cmd.Run())

	return outDir
}

type localURLKind int

const (
	localPage  localURLKind = iota
	localAsset              // CSS or script
)

// pageIDs is a set of IDs found on a page.
type pageIDs map[string]struct{}

// Has returns true if the given ID exists in the set.
func (p pageIDs) Has(id string) bool {
	_, ok := p[id]
	return ok
}

// Add adds an ID to the set.
func (p pageIDs) Add(id string) {
	p[id] = struct{}{}
}

type localURL struct {
	// Kind is the kind of this URL.
	Kind localURLKind

	// URL of the page that linked to this URL.
	// If any.
	From *url.URL

	// Href is the value of the href or src attribute
	// that led to this link.
	Href string

	// Fragment is the fragment identifier from the URL.
	Fragment string

	URL *url.URL
}

func (u localURL) String() string {
	var s strings.Builder
	s.WriteString("localURL{")
	if u.Kind == localPage {
		s.WriteString("page ")
	} else {
		s.WriteString("asset ")
	}
	s.WriteString(u.URL.String())
	if u.From != nil {
		s.WriteString(" from ")
		s.WriteString(u.From.String())
	}
	s.WriteString("}")
	return s.String()
}

type visitOptions struct {
	// Called before each URL is visited.
	// If it returns false, the URL and its children are skipped.
	ShouldVisit func(localURL) bool

	StartPage string // defaults to "/"
	Basename  string // basename of index file (e.g., "index.html" or "_index.html")
}

// visitLocalURLs visits all local URLs in the given directory.
// It does so by spinning up a local HTTP server
// and visiting every page.
//
// 'visit' is called before each URL is visited.
func visitLocalURLs(t *testing.T, root string, opts *visitOptions) {
	if opts == nil {
		opts = new(visitOptions)
	}
	if opts.ShouldVisit == nil {
		opts.ShouldVisit = func(localURL) bool { return true }
	}
	if opts.StartPage == "" {
		opts.StartPage = "/"
	}

	srv := httptest.NewServer(http.FileServer(http.FS(os.DirFS(root))))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	u = u.JoinPath(opts.StartPage)

	(&urlWalker{
		t:           t,
		seen:        make(map[string]struct{}),
		pageIDs:     make(map[string]pageIDs),
		client:      http.DefaultClient,
		shouldVisit: opts.ShouldVisit,
		basename:    opts.Basename,
	}).Walk(u.String())
}

// urlWalker visits all local pages for the generated website
// and verifies that none of the links are broken.
type urlWalker struct {
	t       *testing.T
	host    string
	seen    map[string]struct{}
	pageIDs map[string]pageIDs // IDs found on each page path
	queue   ring.Q[localURL]
	client  *http.Client

	shouldVisit func(localURL) bool
	basename    string // basename of index file (e.g., "index.html" or "_index.html")
}

func (w *urlWalker) Walk(startPage string) {
	u, err := url.Parse(startPage)
	require.NoError(w.t, err)
	w.host = u.Host

	w.queue.Push(localURL{
		Kind: localPage,
		Href: "/",
		URL:  u,
	})
	for !w.queue.Empty() {
		w.visit(w.queue.Pop())
	}
}

func (w *urlWalker) visit(dest localURL) {
	urlString := dest.URL.String()
	if _, ok := w.seen[urlString]; ok {
		return
	}
	w.seen[urlString] = struct{}{}

	if !w.shouldVisit(dest) {
		return
	}

	w.t.Log("Visiting", urlString)
	res, err := w.client.Get(urlString)
	if !assert.NoError(w.t, err, "error visiting %v", dest) {
		return
	}
	defer func() {
		assert.NoError(w.t, res.Body.Close(), "error closing response body")
	}()
	if !assert.Equal(w.t, 200, res.StatusCode, "bad response from %v: %v", dest, res.Status) {
		return
	}

	if path.Ext(dest.Href) == ".css" {
		_, err := io.ReadAll(res.Body)
		assert.NoError(w.t, err, "error reading %v", dest)
		return
	}

	doc, err := html.Parse(res.Body)
	require.NoError(w.t, err)

	// Extract all IDs from this page.
	ids := make(pageIDs)
	for _, el := range cascadia.QueryAll(doc, cascadia.MustCompile("[id]")) {
		for _, attr := range el.Attr {
			if attr.Key == "id" {
				ids.Add(attr.Val)
				break
			}
		}
	}
	w.pageIDs[dest.URL.Path] = ids

	// Validate fragment if this URL has one.
	if dest.Fragment != "" {
		assert.True(w.t, ids.Has(dest.Fragment),
			"fragment #%s not found on page %v (linked from %v)",
			dest.Fragment, dest.URL.Path, dest.From)
	}

	for _, tag := range cascadia.QueryAll(doc, cascadia.MustCompile("script, link, a")) {
		kind, dstAttr := localPage, "href"
		switch tag.Data {
		case "link":
			kind = localAsset
		case "script":
			kind = localAsset
			dstAttr = "src"
		}

		var href string
		for _, attr := range tag.Attr {
			if attr.Key == dstAttr {
				href = attr.Val
				break
			}
		}
		if len(href) != 0 {
			w.push(dest, kind, href)
		}
	}
}

func (w *urlWalker) push(from localURL, kind localURLKind, href string) {
	u, err := url.Parse(href)
	if !assert.NoError(w.t, err, "bad href %q on page %v", href, from.URL) {
		return
	}

	// Capture fragment identifier.
	fragment := u.Fragment

	if len(u.Host) > 0 {
		if u.Host == w.host {
			w.queue.Push(localURL{
				Kind:     kind,
				Href:     href,
				Fragment: fragment,
				URL:      u,
				From:     from.URL,
			})
		}
		return
	}

	// Fragment-only links (e.g., "#pkg-index") refer to the current page.
	if u.Path == "" {
		w.queue.Push(localURL{
			Kind:     kind,
			Href:     href,
			Fragment: fragment,
			URL:      from.URL,
			From:     from.URL,
		})
		return
	}

	// For relative links, resolve them relative to the directory of the current page.
	// If the current page is an index file, get its directory first.
	// Example: /go.abhg.dev/doc2go/_index.html => /go.abhg.dev/doc2go/
	basePath := from.URL.Path
	if w.basename != "" && strings.HasSuffix(basePath, w.basename) {
		basePath = path.Dir(basePath)
	}

	// Join the relative path with the base directory.
	// Example: /go.abhg.dev/doc2go/ + internal/godoc/_index.html
	// => /go.abhg.dev/doc2go/internal/godoc/_index.html
	resolvedPath := path.Join(basePath, u.Path)

	// Create a new URL with the resolved path.
	resolvedURL := *from.URL
	resolvedURL.Path = resolvedPath

	w.queue.Push(localURL{
		Kind:     kind,
		Href:     href,
		Fragment: fragment,
		URL:      &resolvedURL,
		From:     from.URL,
	})
}

// TestModuleVersionLinks verifies that versioned links are generated
// for external dependencies in single-module projects.
func TestModuleVersionLinks(t *testing.T) {
	t.Parallel()

	dir := generate(t, "-C=integration/testdata/single-module", "./...")

	indexHTML := filepath.Join(dir, "example.com", "testpkg", "index.html")
	doc := readHTMLFile(t, indexHTML)

	var foundZap, foundTestify bool
	for href := range listAllHrefs(doc) {
		if strings.Contains(href, "go.uber.org/zap@v1.27.1") {
			foundZap = true
		}
		if strings.Contains(href, "github.com/stretchr/testify@v1.8.4") {
			foundTestify = true
		}
	}

	assert.True(t, foundZap, "expected versioned link to go.uber.org/zap@v1.27.1")
	assert.True(t, foundTestify, "expected versioned link to github.com/stretchr/testify@v1.8.4")
}

// TestModuleVersionLinks_NoVersionsFlag verifies that
// the -no-mod-versions flag disables versioned links.
func TestModuleVersionLinks_NoVersionsFlag(t *testing.T) {
	t.Parallel()

	dir := generate(t, "-C=integration/testdata/single-module", "-no-mod-versions", "./...")

	// Read and parse the generated HTML.
	indexHTML := filepath.Join(dir, "example.com", "testpkg", "index.html")
	doc := readHTMLFile(t, indexHTML)

	var foundZap, foundTestify bool
	for href := range listAllHrefs(doc) {
		assert.NotContains(t, href, "@v",
			"found versioned link in href with -no-mod-versions flag: %s", href)

		if strings.Contains(href, "pkg.go.dev/go.uber.org/zap") {
			foundZap = true
		}

		if strings.Contains(href, "pkg.go.dev/github.com/stretchr/testify") {
			foundTestify = true
		}
	}
	assert.True(t, foundZap, "expected unversioned link to go.uber.org/zap")
	assert.True(t, foundTestify, "expected unversioned link to github.com/stretchr/testify")
}

// TestMultiModuleWorkspace verifies that different modules
// in a workspace can have different versions of the same dependency.
func TestMultiModuleWorkspace(t *testing.T) {
	t.Parallel()

	// Use 'go list -m' to get the modules in the workspace.
	workspaceDir := filepath.Join("integration", "testdata", "multi-module-workspace")
	cmd := exec.Command("go", "list", "-m")
	cmd.Dir = filepath.Join(*_rundir, workspaceDir)
	output, err := cmd.Output()
	require.NoError(t, err, "failed to list modules in workspace")

	modules := strings.Split(strings.TrimSpace(string(output)), "\n")
	require.NotEmpty(t, modules, "no modules found in workspace")

	dir := generate(t, append([]string{"-C=" + workspaceDir}, modules...)...)

	moduleAHTML := filepath.Join(dir, "example.com", "workspace", "modulea", "index.html")
	moduleABody, err := os.ReadFile(moduleAHTML)
	require.NoError(t, err)

	// module A uses zap@v1.27.1 and sync@v0.10.0.
	assert.Contains(t, string(moduleABody),
		"https://pkg.go.dev/go.uber.org/zap@v1.27.1",
		"modulea should link to zap@v1.27.1")
	assert.Contains(t, string(moduleABody),
		"https://pkg.go.dev/golang.org/x/sync@v0.10.0",
		"modulea should link to sync@v0.10.0")

	moduleBHTML := filepath.Join(dir, "example.com", "workspace", "moduleb", "index.html")
	moduleBBody, err := os.ReadFile(moduleBHTML)
	require.NoError(t, err)

	// module B uses zap@v1.26.0 and sync@v0.9.0 (different versions).
	assert.Contains(t, string(moduleBBody),
		"https://pkg.go.dev/go.uber.org/zap@v1.26.0",
		"moduleb should link to zap@v1.26.0 (NOT v1.27.1)")
	assert.Contains(t, string(moduleBBody),
		"https://pkg.go.dev/golang.org/x/sync@v0.9.0",
		"moduleb should link to sync@v0.9.0 (NOT v0.10.0)")
}

// TestReplaceDirectives verifies that replace directives in go.mod
// override the version we link to for external dependencies.
func TestReplaceDirectives(t *testing.T) {
	t.Parallel()

	dir := generate(t, "-C=integration/testdata/replace-directives", "./...")

	// Read the generated HTML.
	indexHTML := filepath.Join(dir, "example.com", "replacepkg", "index.html")
	htmlBody, err := os.ReadFile(indexHTML)
	require.NoError(t, err)

	// go.mod requires zap@v1.27.1 but replaces it with v1.26.0.
	// Links should use the replaced version.
	assert.Contains(t, string(htmlBody),
		"https://pkg.go.dev/go.uber.org/zap@v1.26.0",
		"should link to replaced version zap@v1.26.0")
	assert.NotContains(t, string(htmlBody),
		"https://pkg.go.dev/go.uber.org/zap@v1.27.1",
		"should NOT link to original version zap@v1.27.1")

	// golang.org/x/text has no replace, should use required version.
	assert.Contains(t, string(htmlBody),
		"https://pkg.go.dev/golang.org/x/text@v0.14.0",
		"should link to required version of golang.org/x/text")
}

func readHTMLFile(t *testing.T, filepath string) *html.Node {
	t.Helper()

	htmlFile, err := os.Open(filepath)
	require.NoError(t, err)
	defer func() { _ = htmlFile.Close() }()

	doc, err := html.Parse(htmlFile)
	require.NoError(t, err)

	return doc
}

func listAllHrefs(doc *html.Node) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, tag := range cascadia.QueryAll(doc, cascadia.MustCompile("a[href]")) {
			for _, attr := range tag.Attr {
				if attr.Key == "href" {
					if !yield(attr.Val) {
						return
					}
					break
				}
			}
		}
	}
}
