---
title: Getting started
weight: 2
description: >-
  Start using doc2go from zero.
---

## Steps

1. To get started with doc2go, we need to first install it.

   If you have Go installed,
   run the following command to install doc2go from source:

    ```bash
    go install go.abhg.dev/doc2go@latest
    ```

    If you don't have Go installed,
    or you prefer an alternative installation method,
    see [Installation]({{< relref "install" >}})
    and come back here after installing.

2. Open up a local Go project in your terminal.

   If you don't have one handy,
   run the following command to check out doc2go itself:

    ```bash
    git clone https://github.com/abhinav/doc2go
    cd doc2go
    ```

3. Inside the project directory,
   run the following command to generate its API reference.

    ```bash
    doc2go -out www ./...
    ```

    This will generate a directory named "www".

4. Start a temporary HTTP file server inside the new directory:

    ```bash
    cd www && python -m http.server 8000
    ```

5. Open up <http://127.0.0.1:8000/> in your browser.
   You should be able to browse the documentation for the project.

## Next steps

The above generates a standalone website with doc2go.
The result is ready to use, customize, or deploy.

Next, try the following:

- [Publish the standalone website to GitHub Pages]({{< relref "/docs/publish/github-pages" >}})
- [Explore usage further]({{< relref "usage" >}})
  and [customize doc2go's output]({{< relref "/docs/usage#changing-the-output" >}})
- [Embed the documentation]({{< relref "/docs/embed" >}}) into a larger website
  powered by a static site generator
  like [Jekyll]({{< relref "/docs/embed/jekyll" >}})
  or [Hugo]({{< relref "/docs/embed/hugo" >}})
