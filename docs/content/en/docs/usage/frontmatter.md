---
title: Front matter
description: >-
  Add front matter to generated pages.
weight: 2
---

## Background

Static site generators like Hugo and Jekyll
expect front matter at the top of the file.

This front matter typically takes the form of
a block of YAML delimited by `---`,
or a block of TOML delimited by `+++`.
The block is expected at the top of the file,
and is separated from the rest of the content
by an empty line.

{{< cardpane >}}
{{< card code=true header="YAML" lang="md" >}}
---
title: Getting started
type: docs
no_toc: true
---

To get started [..]
{{< /card >}}

{{< card code=true header="TOML" lang="md" >}}
+++
title = "Getting started"
type = "docs"
no_toc = true
+++

To get started [..]
{{< /card >}}
{{< /cardpane >}}

## Front matter in doc2go

You can use the `-frontmatter` flag of doc2go
to add custom front matter to all generated pages.
This is typically only done in [embedded mode]({{< relref "/docs/embed" >}}).

```bash
doc2go -embed -frontmatter frontmatter.tmpl ./...
```

The argument to `-frontmatter` is a file that contains
a Go [text/template](https://pkg.go.dev/text/template).
The template must include the `---` or `+++` symbols
that delimit the front matter block.

{{< tabpane persist=false >}}
{{< tab header="YAML" lang="plain" >}}
---
# ...
---
{{< /tab >}}
{{< tab header="TOML" lang="plain" >}}
+++
# ...
+++
{{< /tab >}}
{{< /tabpane >}}

See the [reference](#template-context-reference)
for all parameters available to the template.

doc2go will execute the template
for each package that it's generating documentation for.
If the result is not blank,
doc2go will include it at the top of the generated file,
separated from the rest of the content with an empty line.

## Common attributes

### Page title

Hugo and Jekyll let you specify the title of the page
with a `title` attribute in the front matter.

You can use the following template
to set the page title accurately for most cases.

{{< tabpane persist=false >}}
{{< tab header="YAML" lang="plain" >}}
title: "{{ with .Name }}{{ . }}{{ else }}Reference{{ end }}"
{{< /tab >}}
{{< tab header="TOML" lang="plain" >}}
title = "{{ with .Name }}{{ . }}{{ else }}Reference{{ end }}"
{{< /tab >}}
{{< /tabpane >}}

This handles a few different cases:

- for non-main packages, use the name of the package
- for main packages, use the name of the binary---determined by the base name
- for directories that aren't Go packages, use the name of the directory
- lastly, for the top-level directory, use the title "Reference"

## Page description

Some templates make use of the `description` attribute
for SEO and directory listings.

Add the following to your template to set this attribute.

{{< tabpane persist=false >}}
{{< tab header="YAML" lang="plain" >}}
{{ with .Package.Synopsis -}}
  description: {{ printf "%q" . }}
{{ end }}
{{< /tab >}}
{{< tab header="TOML" lang="plain" >}}
{{ with .Package.Synopsis -}}
  description = {{ printf "%q" . }}
{{ end }}
{{< /tab >}}
{{< /tabpane >}}

## Template context reference

doc2go runs the front matter template
with the following context:

```go
struct {
	// Path to the package or directory relative
	// to the module root.
	// This is empty for the root index page.
	Path string
	// Last component of Path.
	// This is empty for the root index page.
	Basename string
	// Number of packages or directories directly under Path.
	NumChildren int

	// The following fields are set only for packages.
	Package struct {
		// Name of the package. Empty for directories.
		Name string
		// First sentence of the package documentation,
		// if any.
		Synopsis string
	}

	// Name of this package or directory.
	Name string
}
```
