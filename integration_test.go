package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"testing"

	"github.com/andybalholm/cascadia"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/iotest"
	"golang.org/x/net/html"
)

func TestIntegration_noBrokenLinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc    string
		pattern string
		args    []string
	}{
		{
			desc:    "self",
			pattern: "./...",
		},
		{
			desc:    "self/home",
			pattern: "./...",
			args:    []string{"-home", "go.abhg.dev/doc2go"},
		},
		{
			desc:    "testify",
			pattern: "github.com/stretchr/testify/...",
		},
		{
			desc:    "x-net",
			pattern: "golang.org/x/net/...",
		},
		{
			desc:    "x-tools",
			pattern: "golang.org/x/tools/...",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			outDir := t.TempDir()
			args := append(tt.args, "-out="+outDir, "-debug", "-internal", tt.pattern)

			exitCode := (&mainCmd{
				Stdout: iotest.Writer(t),
				Stderr: iotest.Writer(t),
			}).Run(args)
			require.Zero(t, exitCode)

			srv := httptest.NewServer(http.FileServer(http.FS(os.DirFS(outDir))))
			t.Cleanup(srv.Close)

			w := newURLWalker(t)
			w.Walk(srv.URL)
		})
	}
}

// urlWalker visits all local pages for the generated website
// and verifies that none of the links are broken.
type urlWalker struct {
	t      *testing.T
	host   string
	seen   map[string]struct{}
	queue  []*url.URL
	client *http.Client
}

func newURLWalker(t *testing.T) *urlWalker {
	return &urlWalker{
		t:      t,
		seen:   make(map[string]struct{}),
		client: http.DefaultClient,
	}
}

func (w *urlWalker) Walk(startPage string) {
	u, err := url.Parse(startPage)
	require.NoError(w.t, err)
	w.host = u.Host

	w.queue = append(w.queue, u)
	for len(w.queue) > 0 {
		var u *url.URL
		u, w.queue = w.queue[0], w.queue[1:]
		w.visit(u)
	}
}

func (w *urlWalker) visit(dest *url.URL) {
	if _, ok := w.seen[dest.String()]; ok {
		return
	}
	w.seen[dest.String()] = struct{}{}

	w.t.Log("Visiting", dest)
	res, err := w.client.Get(dest.String())
	if !assert.NoError(w.t, err, "error visiting %v", dest) {
		return
	}
	defer res.Body.Close()
	if !assert.Equal(w.t, 200, res.StatusCode, "bad response from %v: %v", dest, res.Status) {
		return
	}

	if path.Ext(dest.Path) == ".css" {
		_, err := io.ReadAll(res.Body)
		assert.NoError(w.t, err, "error reading %v", dest)
		return
	}

	doc, err := html.Parse(res.Body)
	require.NoError(w.t, err)

	for _, tag := range cascadia.QueryAll(doc, cascadia.MustCompile("script, link, a")) {
		dstAttr := "href"
		if tag.Data == "script" {
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
			w.push(dest, href)
		}
	}
}

func (w *urlWalker) push(from *url.URL, href string) {
	u, err := url.Parse(href)
	if !assert.NoError(w.t, err, "bad href %q on page %v", href, from) {
		return
	}

	if len(u.Host) > 0 {
		if u.Host == w.host {
			w.queue = append(w.queue, u)
		}
		return
	}

	w.queue = append(w.queue, from.JoinPath(u.Path))
}
