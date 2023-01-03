# doc2go

doc2go is a command line tool
that generates static HTML documentation from your Go code.
It is a self-hosted static alternative to
https://pkg.go.dev/ and https://godocs.io/.

## Installation

Build and install doc2go from source by running:

```bash
go install go.abhg.dev/doc2go@latest
```

## Getting Started

1. Run the following in your terminal to install doc2go from source:

    ```bash
    go install go.abhg.dev/doc2go@latest
    ```

2. Open up a local Go project.
   If you don't have one handy,
   run the following command to check out doc2go itself:

    ```bash
    git clone https://github.com/abhinav/doc2go
    cd doc2go
    ```

3. Inside the project directory,
   run the following command to generate its API reference documentation.

    ```bash
    doc2go -out www ./...
    ```

    This will generate a directory named "www".

4. Start a temporary HTTP file server inside the new directory:

    ```bash
    cd www && python -m http.server 8000
    ```

5. Open up http://127.0.0.1:8000/ in your browser.

## Why

doc2go aims to enable, but is not limited to,
the following use cases:

* self-hosting documentation for your packages
* distributing static documentation websites alongside your software
* deploying your Go API reference with your project's user guide
* adding custom branding to your Go documentation

## Non-goals

doc2go does not aim to
introduce new syntax to the Go documentation syntax.
All comments processed by doc2go must remain valid Go documentation
that can be read with the official tooling.

## License

This software is licensed under the Apache 2.0 License
with the exception of the following files.

    internal/godoc/synopsis.go
    internal/godoc/synopsis_test.go

The license for those files is noted inside them.
