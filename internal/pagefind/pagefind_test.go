package pagefind

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/iotest"
)

var (
	// Directory containing the fake pagefind binary.
	// Set in TestMain.
	_fakeBinDir string

	_fakePagefind string
)

func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == "pagefind" {
		var args pagefindArgs
		args.Parse(os.Args[1:])

		behavior := os.Getenv("TEST_PAGEFIND_BEHAVIOR")
		f, ok := _fakePagefindBehaviors[behavior]
		if !ok {
			log.Fatalf("unknown behavior: %q", behavior)
		}

		f(args)
		os.Exit(0)
	}

	testExe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	// Running tests. Set up a fake pagefind binary.
	_fakeBinDir, err = os.MkdirTemp("", "pagefind-bin")
	if err != nil {
		log.Fatal(err)
	}

	_fakePagefind = filepath.Join(_fakeBinDir, "pagefind")
	if runtime.GOOS == "windows" {
		_fakePagefind += ".exe"
	}

	os.Exit(func() (code int) {
		defer func() { _ = os.RemoveAll(_fakeBinDir) }()

		// Symlink the current executable
		// to the fake pagefind binary.
		if err := os.Symlink(testExe, _fakePagefind); err != nil {
			log.Println(err)
			return 1
		}

		return m.Run()
	}())
}

// pagefindArgs is the subset of pagefind arguments
// that we care about for testing.
type pagefindArgs struct {
	Site         string
	OutputSubdir string
	Verbose      bool
}

func (p *pagefindArgs) Parse(args []string) {
	flag := flag.NewFlagSet("pagefind", flag.ExitOnError)
	flag.StringVar(&p.Site, "site", "", "")
	flag.StringVar(&p.OutputSubdir, "output-subdir", "", "")
	flag.BoolVar(&p.Verbose, "verbose", false, "")
	if err := flag.Parse(args); err != nil {
		log.Fatal(err) // unreachable
	}
}

var _fakePagefindBehaviors = map[string]func(pagefindArgs){
	"dump-args": func(args pagefindArgs) {
		argsPath := os.Getenv("TEST_PAGEFIND_ARGS_PATH")
		if argsPath == "" {
			log.Fatal("TEST_PAGEFIND_ARGS_PATH not set")
		}

		bs, err := json.Marshal(args)
		if err != nil {
			log.Fatal(err)
		}

		if err := os.WriteFile(argsPath, bs, 0o644); err != nil {
			log.Fatal(err)
		}

		log.Printf("wrote args to %s", argsPath)
	},
	"fail": func(pagefindArgs) {
		log.Fatal("fake pagefind failed")
	},
}

func TestCLISuccess(t *testing.T) {
	t.Setenv("PATH", _fakeBinDir)

	tmpDir := t.TempDir()

	tests := []struct {
		desc string
		give IndexRequest
		want pagefindArgs
	}{
		{
			desc: "basic",
			give: IndexRequest{
				SiteDir: tmpDir,
			},
			want: pagefindArgs{
				Site:    tmpDir,
				Verbose: true,
			},
		},
		{
			desc: "output subdir",
			give: IndexRequest{
				SiteDir:     tmpDir,
				AssetSubdir: "assets",
			},
			want: pagefindArgs{
				Site:         tmpDir,
				OutputSubdir: "assets",
				Verbose:      true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			argsPath := filepath.Join(t.TempDir(), "args.json")
			t.Setenv("TEST_PAGEFIND_BEHAVIOR", "dump-args")
			t.Setenv("TEST_PAGEFIND_ARGS_PATH", argsPath)

			c := CLI{
				Log: log.New(iotest.Writer(t), "", 0),
			}
			require.NoError(t, c.Index(context.Background(), tt.give))

			bs, err := os.ReadFile(argsPath)
			require.NoError(t, err)

			var got pagefindArgs
			require.NoError(t, json.Unmarshal(bs, &got))

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCLIFailure(t *testing.T) {
	t.Setenv("PATH", _fakeBinDir)
	t.Setenv("TEST_PAGEFIND_BEHAVIOR", "fail")

	c := CLI{
		Pagefind: _fakePagefind,
	}

	err := c.Index(context.Background(), IndexRequest{
		SiteDir: t.TempDir(),
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "pagefind:")
}
