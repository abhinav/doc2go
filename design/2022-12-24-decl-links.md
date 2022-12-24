# Links inside declarations

## Requirements

Given a snippet like the following in the generated documentation,

```go
type Foo struct {
  Bar Bar
  Baz string
  Qux whatever.Quux
}
```

There are a couple basic requirements:

* We expect links for each of the field types.
  * The second 'Bar' in `Bar Bar` is linked to '#Bar'.
  * The 'string' is linked to 'https://pkg.go.dev/builtin#string'.
  * In `whatever.Quux`,
    'whatever' is linked to the documentation for the 'whatever' package,
    and 'Quux' is linked to that page plus '#Quux'.
* We expect anchors for each of the field names,
  concatenated with the struct name.
  So the Baz field gets,

    ```
    <a id="Foo.Bar>Bar</a>
    ```

The same expectations are extrapolated to other declarations
in the documentation with the following notes:

* Interfaces are similar to structs, except the methods get anchors.
* All types -- local and external -- referenced in any function signature
  must be linked.
* Generic parameters *are not linked* (yet)

Top-level exported entities do not get anchors
because they're covered by the corresponding headers
in the generated HTML.
This is true for all except variables and constants:
they get anchors because
they're dumped into either a "Variables" or a "Constants" section.

## Implementation

gddo and pkgsite both do this differently,
but the overall idea is the same:

- Traverse the decl AST in the same order as identifiers appear in the text,
  and for each identifier, record whether it's a declaration or a reference.
  pkgsite does this by generating "anchor points" and "anchor links"
  for all identifiers,
  and gddo does this by generating "annotations" for all identifiers.
- Format the decl with go/printer or go/format,
  and use go/scanner to scan through it.
  The scanner will encounter identifiers in the same order as the traversal.
  This will allow correlating sections of the formatted source
  to whether they should have a link or an anchor around them.
- Afterwards, the collected information can be rendered to HTML.

We follow a similar approach.
