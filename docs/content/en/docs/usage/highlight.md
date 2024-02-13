---
title: Syntax highlighting
description: >-
  Customize doc2go's syntax highlighting of code blocks.
weight: 3
---

doc2go highlights code inside API reference declarations
with the help of the [Chroma][chroma] library.
This keeps syntax highlighting "server-side" as opposed to "client side"---
the HTML it generates already contains syntax highlighting instructions.

  [chroma]: https://github.com/alecthomas/chroma

## Themes

Themes specify the style for syntax highlighting,
answering questions like *what* should be rendered
when a comment is encountered.

### Changing themes

Use the `-highlight` flag to change the theme for syntax highlighting.

```bash
doc2go -highlight monokai ./...
```

### Available themes

Get a full list of themes supported by doc2go
by running the following command:

```bash
doc2go --highlight-list-themes
```

doc2go supports all themes that come with [Chroma][chroma]
as well as a custom theme named 'plain'
intended to mirror the minimal syntax highlighting on https://pkg.go.dev/.

```bash
doc2go -highlight plain ./...
```

You can experiment with all but 'plain' at
[Chroma Playground](https://swapoff.org/chroma/playground/)

## Highlighting modes

Where the theme specifies *what* will be rendered when highlighting,
highlighting modes determine **how** it will be rendered.

doc2go supports the following highlight modes:

| Mode      | Description                                                              |
|-----------|--------------------------------------------------------------------------|
| `inline`  | Highlighting is performed by inline style attributes on the page         |
| `classes` | Highlighting is performed by a CSS style sheet included in the page      |
| `auto`    | One of the other two methods is picked based on other command line flags |

The default is `auto`. It currently behaves as follows,
however, this is subject to change:

> Without the `-embed flag`, `auto` means `classes`.
> With the `-embed` flag, `auto` means `inline`.

The effect of this is that syntax highlighting works out of the box
in both standalone and embedded modes without any additional setup.

### Changing highlighting modes

You can change the highlighting mode with the `-highlight` flag
by supplying the name followed by a colon (`:`).

```bash
doc2go -highlight inline: ./...
```

{{% alert title="Note" %}}
The colon (`:`) is necessary.
Without that, doc2go will assume you're [changing the theme](#changing-themes).
{{% /alert %}}


If you'd like to also change the theme, add it after the colon suffix.

```bash
doc2go -highlight classes:tango ./...
```

The following are all valid uses of the `-highlight` flag.

```bash
# Use inline highlighting and the default theme.
doc2go -highlight inline: # ...

# Use class-based highlighting with the tango theme.
doc2go -highlight classes:tango # ...

# Use the default highlighting mode with the github theme.
doc2go -highlight github # ...
```

### Printing the theme CSS

The `classes` highlight mode works only if the style sheet of the theme
is included in the page.
doc2go does this by default in standalone mode (without the `-embed` flag).

To access the style sheet for a theme, use the `-theme-print-css` flag:

```bash
# Get the default theme's style sheet:
doc2go -highlight-print-css

# Get a specific theme's style sheet:
doc2go -highlight dracula -highlight-print-css
```

## See Also

- [Syntax highlighting with CSS]({{< relref "/docs/embed#syntax-highlighting-with-css" >}})
