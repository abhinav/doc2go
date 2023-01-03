---
title: Usage
weight: 4
description: >-
  Learn how to use doc2go.
---

## Generating documentation

The simplest form of using doc2go is as follows:

```bash
doc2go ./...
```

When run inside a Go module,
this will generate documentation for the current package
and all its descendants
into the "_site" directory.

The generated website is standalone,
and can be [deployed]({{< relref "/docs/publish" >}})
to your chosen web host as-is.

## Specifying the input

doc2go expects one or more **import path patterns**.

`./...` is shorthand for the package in the current directory
and its descendants.
You can specify one or more import paths explicitly to generate
documentation for those packages.

```bash
doc2go github.com/yuin/goldmark github.com/yuin/goldmark/ast
```

Add the `/...` suffix to an import path to generate documentation
for that package and all its descendants.

```bash
doc2go go.uber.org/zap/...
```

### Third-party packages

You can generate documentation for third-party packages with doc2go.

To do this, pass in their module paths to the command,
and suffix each with `/...`.

```bash
doc2go go.uber.org/zap/... github.com/rs/zerolog/...
```

All specified package must be present in your current project's go.mod.

### Standard library

doc2go can generate the API reference for the Go standard library
with the following command:

```bash
doc2go std
```

## Changing the output

### Output directory

Add an `-out` flag if you prefer something other than "_site".

```bash
doc2go -out public ./...
```

The directory will be created if it doesn't exist.

#### Base name

All generated pages use the name "index.html".
For example, the documentation for `encoding/json`
will go into `_site/encoding/json/index.html`.

You can change this to something else with the `-basename` flag.

```bash
doc2go -basename index.htm ./...
```

Use this to make doc2go's output compatible with Hugo.

```bash
doc2go -basename _index.html # other flags ...
```

See [Embedding into Hugo]({{< relref "/docs/embed/hugo" >}}) for more.


### Internal packages

doc2go generates documentation for all packages
that match the specified patterns.
In this documentation, it includes a list of subpackages for a package
at the bottom of the documentation for that package.

By default, internal packages are not included in the list of subpackages,
but their documentation is still there.
That is, given `example.com/foo/internal`,
doc2go will generate documentation the package,
but it will not list as a subpackage of `example.com/foo`.
If a user knows it's there, they'll be able to visit it.

Use doc2go's `-internal` flag to include internal packages
in the subpackage listing.

```bash
doc2go -internal ./...
```

With this flag enabled, `example.com/foo/internal` will be listed
as a subpackage of `example.com/foo`.

## CLI Reference

{{< readfile file="usage.txt" code="true" lang="plain" >}}
