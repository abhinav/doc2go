---
title: Frequently asked questions
linkTitle: FAQ
weight: 20
---

If your question isn't answered here,
please [start a discussion](https://github.com/abhinav/doc2go/discussions)
or [create an issue](https://github.com/abhinav/doc2go/issues/new).

## Troubleshooting

This section addresses common issues and their solutions.

### My web host doesn't like "/foo" URLs. They want "/foo/" URLs.

By default, doc2go generates relative links in the form:

```html
<a href="../path/to/dst">...</a>
```

If your web host prefers directories to have a trailing slash,
run doc2go with `-rel-link-style=directory`.

```sh
doc2go -rel-link-style=directory ./...
```

This will generate:

```html
<a href="../path/to/dst/">...</a>
```
