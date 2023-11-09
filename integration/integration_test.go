package integration

import (
	"encoding/json"
	"flag"
	"io"
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
	"golang.org/x/net/html"
)

var (
	_doc2go = flag.String("doc2go", "", "path to doc2go binary")
	_rundir = flag.String("rundir", "", "path to directory to run doc2go in")
)

func Test(t *testing.T) {
	t.Parallel()

	testdatas, err := filepath.Glob("testdata/self.json")
	require.NoError(t, err, "error globbing testdata")
	require.NotEmpty(t, testdatas, "no testdata found")

	if *_doc2go == "" {
		var err error
		*_doc2go, err = exec.LookPath("doc2go")
		require.NoError(t, err, "could not find doc2go binary and -doc2go flag was not set")
	}

	for _, testdata := range testdatas {
		testdata := testdata
		name := strings.TrimSuffix(filepath.Base(testdata), filepath.Ext(testdata))
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f, err := os.Open(testdata)
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, f.Close())
			}()

			var argSets [][]string
			dec := json.NewDecoder(f)
			for dec.More() {
				var args []string
				require.NoError(t, dec.Decode(&args))
				argSets = append(argSets, args)
			}

			for _, args := range argSets {
				args := args
				t.Run(strings.Join(args, " "), func(t *testing.T) {
					t.Parallel()

					testIntegration(t, args)
				})
			}
		})
	}
}

func testIntegration(t *testing.T, args []string) {
	root := t.TempDir()
	outDir := root

	args = append([]string{"-out=" + outDir, "-internal", "-debug"}, args...)

	cmd := exec.Command(*_doc2go, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = *_rundir
	require.NoError(t, cmd.Run())

	srv := httptest.NewServer(http.FileServer(http.FS(os.DirFS(root))))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	w := newURLWalker(t)
	w.Walk(u.String())
}

// urlWalker visits all local pages for the generated website
// and verifies that none of the links are broken.
type urlWalker struct {
	t      *testing.T
	host   string
	seen   map[string]struct{}
	queue  ring.Q[*url.URL]
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

	w.queue.Push(u)
	for !w.queue.Empty() {
		w.visit(w.queue.Pop())
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
	defer func() {
		assert.NoError(w.t, res.Body.Close(), "error closing response body")
	}()
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
			w.queue.Push(u)
		}
		return
	}

	w.queue.Push(from.JoinPath(u.Path))
}
