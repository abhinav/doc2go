---
title: Configuring external links
description: >-
  Change how doc2go generates links to external repositories.
---

By default, doc2go will generate links to other packages
using the following logic:

> If the package is inside the current generation scope,
> doc2go will generate a relative link to it.
>
> Everything else will use <https://pkg.go.dev>.

For example, if you run `doc2go example.com/foo/...`,
doc2go will use `../b` for
a link from `example.com/foo/a` to `example.com/foo/b`,
and it'll use `https://pkg.go.dev/example.com/bar`
for a link to `example.com/bar`.

This works well but it doesn't handle dependencies
that host their documentation elsewhere---possibly using doc2go.

You can use the `-pkg-doc` flag to change this.

## Specifying templates

The `-pkg-doc` flag takes the form:

```
-pkg-doc PATH=TEMPLATE
```

Where `PATH` is an import path (e.g. `example.com/foo`)
and `TEMPLATE` is a Go [text/template](https://pkg.go.dev/text/template).
Given the flag,
doc2go will use `TEMPLATE` to generate links to documentation
for the package at `PATH` and all its descendants.
See the [reference](#template-context-reference)
for all parameters available to the template.

For example:

```bash
-pkg-doc example.com/bar='https://go.example.com/{{.ImportPath}}'
```

This will generate the following links:

  | Package | Link |
  |---------|------|
  | `example.com/bar` | `https://go.example.com/example.com/bar` |
  | `example.com/bar/baz` | `https://go.example.com/example.com/bar/baz` |
  | `example.com/foo` | `https://pkg.go.dev/example.com/bar/baz` |

## Multiple import paths

You can pass the flag multiple times for different import path scopes:

```bash
-pkg-doc example.com='https://godocs.io/{{.ImportPath}}' \
-pkg-doc go.abhg.dev/doc2go='https://abhinav.github.io/doc2go/api/{{.ImportPath}}'
```

The above will use `godocs.io` for everything under `example.com`,
and for doc2go's own documentation, it'll link to this website.

### Overlapping import paths

If paths specified in different `-pkg-doc` flags overlap,
doc2go will use the template from the more specific of the two.

For example, suppose `example.com/foo/bar` is a submodule of `example.com/foo`,
with its documentation hosted elsewhere.
We can use the following:

```bash
-pkg-doc example.com/foo='https://godocs.io/{{.ImportPath}}' \
-pkg-doc example.com/foo/bar='https://go.example.com/{{.ImportPath}}'
```

## Template context reference

doc2go runs the pkg-doc template
with the following context:

```go
struct {
	// Import path of the target package.
	ImportPath string
}
```
