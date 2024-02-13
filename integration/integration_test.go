package integration

import (
	"flag"
	"io"
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
		name string
		args []string
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := generate(t, tt.args...)
			visitLocalURLs(t, dir, nil)
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
		tt := tt
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
			continue
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

type localURL struct {
	// Kind is the kind of this URL.
	Kind localURLKind

	// URL of the page that linked to this URL.
	// If any.
	From *url.URL

	// Href is the value of the href or src attribute
	// that led to this link.
	Href string

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
		client:      http.DefaultClient,
		shouldVisit: opts.ShouldVisit,
	}).Walk(u.String())
}

// urlWalker visits all local pages for the generated website
// and verifies that none of the links are broken.
type urlWalker struct {
	t      *testing.T
	host   string
	seen   map[string]struct{}
	queue  ring.Q[localURL]
	client *http.Client

	shouldVisit func(localURL) bool
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

	if len(u.Host) > 0 {
		if u.Host == w.host {
			w.queue.Push(localURL{
				Kind: kind,
				Href: href,
				URL:  u,
				From: from.URL,
			})
		}
		return
	}

	w.queue.Push(localURL{
		Kind: kind,
		Href: href,
		URL:  from.URL.JoinPath(u.Path),
		From: from.URL,
	})
}
