---
title: Embedding into Jekyll
linkTitle: Jekyll
description: >-
   Use doc2go to generate output compatible with Jekyll.
---


To feed the output of doc2go into Jekyll, you need at least the following:

- enable embedded mode
- add front matter to each generated file
- generate pages into your Jekyll directory

So if your Jekyll website is a subdirectory inside your project
with a layout similar to the following:

```
Project root
 |- go.mod
 |- foo.go
 '- docs/
     |- _config.yml
     |- _posts/
     '- index.md
```

Add a [front matter template]({{< relref "/docs/usage/frontmatter" >}})
to the docs directory:

```bash
cat > docs/frontmatter.tmpl << EOF
---
title: "{{ with .Name }}{{ . }}{{ else }}Reference{{ end }}"
layout: default
render_with_liquid: false
---
EOF
```

And run this command from the project root:

```bash
doc2go -embed -out docs/api  -frontmatter docs/frontmatter.tmpl ./...
```

This will generate your API reference under an `/api` path
on your website.

The front matter template above specifies that your website
should use the package or binary name as the page title,
or the directory name if it's not a Go package.
Additionally, it instructs Jekyll's templating system
([Liquid](https://shopify.github.io/liquid/))
to ignore these pages---this prevents Liquid
from interpreting occurences of `{{`, `}}`, `{%`, and `%}`
in your documentation as Liquid filters or tags.

See [Adding front matter]({{< relref "/docs/usage/frontmatter" >}})
to write your own templates.
