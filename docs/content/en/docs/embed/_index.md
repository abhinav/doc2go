---
title: Embedding
description: >-
  Embed the output of doc2go into another website.
weight: 5
---

This section covers how to feed the output of doc2go
into static site generators
so that your API reference is embedded into a larger website.

![doc2go reads Go, generates partial HTML](../embedded-flow.png)

Typically, you need the following flags for embedding:

`-embed`
: This enables the embedding behavior.
  Instead of generating a standalone website
  with its own `<html>` tag and static assets,
  doc2go will generate just the content.

[`-frontmatter`]({{< relref "/docs/usage/frontmatter" >}})
: Many static site generators expect YAML or TOML front matter
  at the top of each file.
  You'll need to craft a front matter template
  specifically for your static site generator and theme.
  See the [section on front matter]({{< relref "/docs/usage/frontmatter" >}})
  to learn how you can craft your own templates.

[`-basename`]({{< relref "/docs/usage#base-name" >}})
: Some static site generators expect index files inside directories
  to be named something other than `index.html`.
  Use this flag to change the name for index files.
  For example, [Hugo]({{< relref "hugo" >}}) expects `_index.html`
  so you'll use `-basename _index.html`.
