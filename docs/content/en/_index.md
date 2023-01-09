---
title: doc2go
---

{{< blocks/cover height="min" title="Welcome to doc2go" color="dark" >}}
<div class="mx-auto">
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
([example](example/go.abhg.dev/doc2go)),
or embed your documentation inside another static website
([example](api/go.abhg.dev/doc2go)).

<a class="btn btn-dark" href="{{< relref "docs/publish/github-pages" >}}">
  <i class="fa-brands fa-github"></i> Publish to GitHub Pages
</a>
<a class="btn btn-dark" href="{{< relref "docs/embed" >}}">
  <i class="fa-solid fa-folder-tree"></i> Embed into another website
</a>

</div>
<div class="col-md-auto">

Usage can be as simple as:

<pre class="bg-light p-1 rounded"><code>mkdir www
doc2go -out www/ ./...</code></pre>

<a class="btn btn-dark" href="{{< relref "docs/usage" >}}">
  Usage <i class="fas fa-arrow-alt-circle-right ml-2"></i>
</a>

</div>
</div>
</div>
{{% /blocks/section %}}

{{% blocks/section color="orange" %}}
{{% blocks/feature title="Easy to host" icon="fa-solid fa-server" %}}
doc2go generates static websites
that you can host on
[GitHub Pages]({{< relref "docs/publish/github-pages" >}})
or any other static website hosting service.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-book" title="Everything in one place" %}}
Embed your API Reference into a bigger static website
with [Hugo]({{< relref "/docs/embed/hugo" >}})
or [Jekyll]({{< relref "/docs/embed/jekyll" >}})
All your documentation in one place, on one website.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-box-archive" title="Take it with you" %}}
Store offline copies of API documentation for projects you use frequently.
Keep using it even with a bad internet connection.
{{% /blocks/feature %}}
{{% /blocks/section %}}
