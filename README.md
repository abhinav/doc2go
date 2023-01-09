# doc2go

doc2go is a command line tool
that generates static HTML documentation from your Go code.
It is a self-hosted static alternative to
https://pkg.go.dev/ and https://godocs.io/.

Documentation for the tool is available at https://abhinav.github.io/doc2go/.

## Installation

See <https://abhinav.github.io/doc2go/docs/install/>,
but in short, use one of the following methods:

```bash
# Homebrew/Linuxbrew:
brew install abhinav/tap/doc2go

# ArchLinux User Repository
yay -S doc2go-bin

# Build from source
go install go.abhg.dev/doc2go@latest
```

## Getting Started

To get started with doc2go, see
<https://abhinav.github.io/doc2go/docs/start/>.

If you just want a copy-paste-friendly setup
to publish your documentation to GitHub Pages,
see <https://abhinav.github.io/doc2go/docs/publish/github-pages/>.

## License

This software is licensed under the Apache 2.0 License
with the exception of the following files.

    internal/godoc/synopsis.go
    internal/godoc/synopsis_test.go

The license for those files is noted inside them.
