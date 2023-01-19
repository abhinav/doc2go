---
title: Configuration
weight: 1
description: >-
  Configure doc2go with configuration files
  instead of command line flags.
---

Configuration for doc2go is read from a file named **doc2go.rc**
expected in the directory where doc2go is running.

## Configuration format

The configuration file is a list of newline-separated options.
Each line is in one of the following forms:

    key
    key value

Where `key` is the name of an option and `value` is the value for it.
The value may contain spaces and punctuation without escaping or quoting.
If the option is a boolean switch, then `value` is not present.

For example:

```
home go.abhg.dev/doc2go
embed
theme inline:tango
```

Lines that start with `#` are treated as comments.

```
# Use godocs.io for example.com/foo.
pkg-doc example.com/foo=https://godocs.io/{{.ImportPath}}
```

### Options

doc2go accepts nearly all [command line flags]({{< relref "/docs/usage#cli-reference" >}})
as configuration parameters.

Flags which accept values are specified alongside them:

    home go.abhg.dev/doc2go

Whereas boolean flags are specified alone on their own lines:

    embed
    internal

See  [CLI Reference]({{< relref "/docs/usage#cli-reference" >}})
for a list of flags.

## Using a different file

Configuration may be stored in a file other than doc2go.rc.
Specify the path to this file while invoking doc2go
to read configuration from it.

```bash
doc2go -config other.rc ./...
```

You can use this flag to have different configurations for
your embedded versus standalone sites, for example.

```bash
doc2go -config embedded.rc ./...
doc2go -config standalone.rc ./...
```
