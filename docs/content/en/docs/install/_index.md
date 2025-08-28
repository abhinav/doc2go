---
title: Installation
weight: 3
description: >-
  Install doc2go on your machine.
---

You can install [pre-built binaries](#binary-installation) of doc2go,
or [install it from source](#install-from-source).

## Binary installation

The following methods are available
to install pre-built binaries of doc2go.

### Homebrew and Linuxbrew

If you're using [Homebrew](https://brew.sh/) on macOS or Linux,
run the following command:

```bash
brew install --cask abhinav/tap/doc2go
```

### ArchLinux

If you're using ArchLinux,
install doc2go from [AUR](https://aur.archlinux.org/)
using the [doc2go-bin](https://aur.archlinux.org/packages/doc2go-bin/)
package.

```bash
git clone https://aur.archlinux.org/doc2go-bin.git
cd doc2go-bin
makepkg -si
```

If you use an AUR helper like [yay](https://github.com/Jguer/yay),
run the following command instead:

```go
yay -S doc2go-bin
```

### GitHub Releases

Download pre-built binaries of doc2go from its
[releases page](https://github.com/abhinav/doc2go/releases).

## Install from source

To install doc2go from source,
first [install Go](https://go.dev/dl/) if you don't already have it,
and then run:

```bash
go install go.abhg.dev/doc2go@latest
```
