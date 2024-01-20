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
	"golang.org/x/net/html"
)

var (
	_doc2go = flag.String("doc2go", "", "path to doc2go binary")
	_rundir = flag.String("rundir", "", "path to directory to run doc2go in")
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
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			visitLocalURLs(t, generate(t, tt.args...), nil)
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
	visitLocalURLs(t, root, func(local localURL) bool {
		if local.Kind != localPage {
			return false
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
	visitLocalURLs(t, outDir, func(local localURL) bool {
		if local.Kind != localPage {
			return false
		}

		path := strings.TrimPrefix(local.URL.Path, "/")
		if root, _, ok := strings.Cut(path, "/"); ok {
			roots[root] = struct{}{}
		}
		return true
	})

	assert.Equal(t, map[string]struct{}{
		"v1.1.0": {},
		"v1.2.3": {},
	}, roots)
}

func generate(t *testing.T, args ...string) (outDir string) {
	// Unless -internal=false is specified, we'll always add -internal.
	var noInternal bool
	for i, arg := range args {
		if v, ok := strings.CutPrefix(arg, "-out="); ok {
			outDir = v
			continue
		}
		if arg == "-out" && i+1 < len(args) {
			outDir = args[i+1]
			continue
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

// visitLocalURLs visits all local URLs in the given directory.
// It does so by spinning up a local HTTP server
// and visiting every page.
//
// 'visit' is called before each URL is visited.
// If it returns false, the URL and its children are skipped.
func visitLocalURLs(t *testing.T, root string, visit func(localURL) bool) {
	if visit == nil {
		visit = func(localURL) bool { return true }
	}

	srv := httptest.NewServer(http.FileServer(http.FS(os.DirFS(root))))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	(&urlWalker{
		t:           t,
		seen:        make(map[string]struct{}),
		client:      http.DefaultClient,
		shouldVisit: visit,
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
