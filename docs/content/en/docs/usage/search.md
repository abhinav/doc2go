---
Title: Search
description: >-
  Add client-side search to your documentation.
weight: 1.5
---

{{% alert title="Note" color="info" %}}
This feature is available in standalone mode only.
If you're not passing the `-embed` flag,
you're in standalone mode.

If you want search in embedded mode,
use the static site generator you're embedding into
to give you search functionality,
or plug [Pagefind](https://pagefind.app) into the generated site.
{{% /alert %}}

doc2go can include a search box in your generated website,
allowing users to perform full-text search across your documentation.
This functionality is powered by [Pagefind](https://pagefind.app),
making this entirely client-side and static.

Try out one of the following examples:

- [doc2go's own documentation](../../../example)
- [Go standard library](../../../std)

## Enabling search

By default, the search functionality is enabled automatically
if pagefind is found installed on your system `$PATH`.
Use the [official installation instructions](https://pagefind.app/docs/installation/)
to install pagefind on your system.

You can explicitly enable or disable search with the `-pagefind` flag:

```bash
doc2go -pagefind       ./...  # enabled
doc2go -pagefind=false ./...  # disabled
```

If you have pagefind installed in a non-standard location,
pass that location instead of true or false:

```bash
doc2go -pagefind=path/to/pagefind ./...
```

## Project-local installation

If you'd like to keep the installation of pagefind local to your project,
you can install it with NPM and pass the location to the `-pagefind` flag.

Take the following steps:

1. Install pagefind with NPM:

    ```bash
    npm install pagefind@latest
    ```

    This will create a `node_modules/.bin/pagefind` executable.

2. Run doc2go with this path as an argument:

    ```bash
    doc2go -pagefind=node_modules/.bin/pagefind ./...
    ```
