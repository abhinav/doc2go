---
title: Embedding into Hugo
linkTitle: Hugo
description: >-
   Use doc2go to generate output compatible with Hugo.
---

To feed the output of doc2go into Hugo,
you need at least the following:

- enable embedded mode
- change the base name of index pages to `_index.html`
- generate the pages into the Hugo `contentDir`---defaults to 'content/'

So if your Hugo website is a subdirectory inside your project
with a layout similar to the following:

```
Project root
 |- go.mod
 |- foo.go
 '- docs/
     |- config.toml
     '- content/
         |- _index.md
         '- other-pages.md
```

Run this command from your project root:

```bash
doc2go -embed \
  -basename _index.html \
  -out docs/content/api \
  ./...
```

This will generate an `api/` subfolder in your Hugo website
that holds your project's API reference.

## Front matter templates

Depending on the theme you're using
you will probably need to provide a front matter template
to make the generated pages compatible with the template.

```bash
doc2go -embed \
  -basename _index.html \
  -out docs/content/api \
  -frontmatter frontmatter.tmpl \
  ./...
```

For example, the theme may require an explicit title,
to set the page type, or other customization.

See [Adding front matter]({{< relref "/docs/usage/frontmatter" >}})
to write your own templates,
or see below for some you can use.

### Reusable front matter templates

Mix and match the following into your front matter template.
Be sure to add  the `---` delimiters at the start and end of the template.

```yaml
---
# ...
---
```

#### Page title

```
title: "{{ with .Name }}{{ . }}{{ else }}Reference{{ end }}"
```

This gives us a title based on the package or binary name,
or the base name of the directory if it's not a Go package.

#### [Docsy](https://docsy.dev/)

```
no_list: true
type: docs
```

- Disables the Docsy-generated list of child pages with
  [`no_list`](https://www.docsy.dev/docs/adding-content/content/#docs-section-landing-pages)
  because doc2go generates its own subpackage listing.
- Sets the [page type](https://www.docsy.dev/docs/adding-content/content/#content-sections-and-templates)
  to treat it as a documentation page.

#### [Hugo Book](https://github.com/alex-shpak/hugo-book)

```
bookToC: false
bookCollapseSection: {{ if .NumChildren }}true{{ else }}false{{ end }}
```

- Disables the auto-generated table-of-contents
  to the right of each page
  because it doesn't recognize HTML headers.
- Defaults to collapsing nested sections in the documentation tree on the left.

{{% alert title="Tip" %}}
Add your own templates here.
Suggest a change using the *Edit this page* link on the right.
{{% /alert %}}
