---
title: doc2go
description: "Your Go documentation, to-go."
---

{{< blocks/cover height="min" title="Welcome to doc2go" color="dark" >}}
<div class="mx-auto">
  <p>
    <img src="logo.png" alt="Gopher on a flying sheet of paper" />
  </p>
  <a class="btn btn-lg btn-light mr-3 mb-4" href="{{< relref "docs/start" >}}">
    Get Started <i class="fas fa-play ml-2"></i>
  </a>
  <a class="btn btn-lg btn-primary mr-3 mb-4" href="{{< relref "docs/install" >}}">
    Installation <i class="fas fa-download ml-2"></i>
  </a>
  <p class="lead ">Your Go documentation, to-go.</p>
</div>
{{< /blocks/cover >}}

{{% blocks/section color="primary" %}}
<div class="container">
<div class="row">
<div class="col col-lg">

doc2go is a tool that generates static HTML documentation from your Go code.
It's a simpler, self-hosted alternative to services like
https://pkg.go.dev/ and https://godocs.io/.

You can use doc2go to generate standalone HTML websites
([example](example/)),
or embed your documentation inside another static website
([example](api/)).

<a class="btn btn-dark" href="{{< relref "docs/publish/github-pages" >}}">
  <i class="fa-brands fa-github"></i> Publish to GitHub Pages
</a>
<a class="btn btn-dark" href="{{< relref "docs/embed" >}}">
  <i class="fa-solid fa-folder-tree"></i> Embed into another website
</a>

</div>
<div class="col-md-auto">

Usage can be as simple as:

<pre class="bg-dark p-2 rounded"><code>mkdir www
doc2go -out www/ ./...</code></pre>

<a class="btn btn-dark" href="{{< relref "docs/usage" >}}">
  Usage <i class="fas fa-arrow-alt-circle-right ml-2"></i>
</a>

</div>
</div>
</div>
{{% /blocks/section %}}

{{% blocks/section color="orange" type="row" %}}
{{% blocks/feature title="Easy to publish" icon="fa-solid fa-server" url="docs/publish" %}}
doc2go generates static websites
that you can host on
[GitHub Pages]({{< relref "docs/publish/github-pages" >}})
or any other static website hosting service.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-book" title="Everything in one place" url="docs/embed" %}}
Embed your API Reference into a bigger static website
with [Hugo]({{< relref "/docs/embed/hugo" >}})
or [Jekyll]({{< relref "/docs/embed/jekyll" >}}).
All your documentation in one place, on one website.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-file-code" title="Syntax highlighting" url="docs/usage/highlight" %}}
Get syntax highlighting in your documentation out of the box.
Choose from over 50 themes.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-brands fa-css3-alt" title="Style it your way" %}}
With [embedding]({{< relref "/docs/embed" >}}),
doc2go gives you full control of your documentation's CSS.
Brand it, style it, go wild.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-box-archive" title="Take it with you" %}}
Store offline copies of API documentation for projects you use frequently.
Keep using it even with a bad internet connection.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-feather-pointed" title="Lightweight" %}}
doc2go is lightweight and composes with other systems
instead of trying to replace them.
{{% /blocks/feature %}}
{{% /blocks/section %}}
