---
title: Publish to GitHub Pages
linkTitle: GitHub Pages
description: >-
   Publish the output of doc2go directly to
   a GitHub Pages website.
---

## Workflow

To generate your API Reference with doc2go
and publish it to GitHub Pages,
take the following steps.

1. In your GitHub project, go to **Settings > Pages**.

2. Set the **Source** to **GitHub Actions**.

    ![Set Source to GitHub Actions](../gh-pages-source.png)

3. In the root of your repository, create a new file at:
   **`.github/workflows/doc2go.yml`**
   with the following contents:

    ```yaml
    name: Publish API Reference

    on:
      # Publish documentation when a new release is tagged.
      push:
        tags: ['v*']

      # Allow manually publishing documentation from a specific hash.
      workflow_dispatch:
        inputs:
          head:
            description: "Git commit to publish documentation for."
            required: true
            type: string

    # If two concurrent runs are started,
    # prefer the latest one.
    concurrency:
      group: "pages"
      cancel-in-progress: true

    jobs:

      build:
        name: Build website
        runs-on: ubuntu-latest
        steps:
          - name: Checkout
            uses: actions/checkout@v3
            with:
              # Check out head specified by workflow_dispatch,
              # or the tag if this fired from the push event.
              ref: ${{ inputs.head || github.ref }}
          - name: Setup Go
            uses: actions/setup-go@v3
            with:
              go-version: stable
              cache: true
          - name: Install doc2go
            run: go install go.abhg.dev/doc2go@latest
          - name: Generate API reference
            run: doc2go ./...
          - name: Upload pages
            uses: actions/upload-pages-artifact@v1

      publish:
        name: Publish website
        # Don't run until the build has finished running.
        needs: build

        # Grants the GITHUB_TOKEN used by this job
        # permissions needed to publish the website.
        permissions:
          pages: write
          id-token: write

        # Deploy to the github-pages environment
        environment:
          name: github-pages
          url: ${{ steps.deployment.outputs.page_url }}

        runs-on: ubuntu-latest
        steps:
          - name: Deploy to GitHub Pages
            id: deployment
            uses: actions/deploy-pages@v1
    ```

    This workflow defines two triggers:

    - `tags`: This publishes the latest documentation
      when a new version of your library is released.
    - `workflow_dispatch`: This is invoked manually
      from the GitHub Pages UI or the GitHub CLI
      to publish documentation from a specific Git commit manually.

4. Commit and push this file.
   This won't yet publish anything
   because you haven't tagged a release yet.

5. Trigger the workflow once manually with the tag
   for the most recent release of your project.
   You can use the GitHub CLI or the UI for this.

    - If you have the GitHub CLI, run the following command:

        ```bash
        VERSION=v1.2.3
        gh workflow run -f head=$VERSION doc2go.yml
        ```

        Be sure to set `VERSION` to your latest release.

    - Otherwise, open your repository on GitHub and then:
      Go to **Actions > Publish API Reference**.
      Click the **Run workflow** button at the top of the list of runs,
      and input the version number for your most recent release.

## Chasing HEAD

The workflow above publishes from the latest tagged version.
This is desirable because it keeps your API reference in sync
with the latest releases.

If, for some reason, you don't tag versions
or would prefer to report documentation from the head
of your main branch,
change the `tags` trigger in the workflow to the following:

```yaml
on:
  push:
    branches: [main]
```

Use `master` above if the name of your main branch is `master`.
