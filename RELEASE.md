# Release process

> This document is intended for doc2go maintainers only.

## Prerequisites

The following must be installed for a new release:

- [GitHub CLI (gh)](https://cli.github.com/)

## Steps

To release a new version of doc2go, take the following steps:

1. Trigger the "Prepare release" workflow.
   This will automatically determine the next version,
   prepare the changelog,
   and create a pull request to `main`.

    ```bash
    gh workflow run prepare-release.yml -f version=minor
    # or "major" or "patch"
    ```

2. Review and merge the pull request created by the workflow.

3. Trigger the "Publish release" workflow to tag and publish the release.

    ```bash
    gh workflow run publish-release.yml -f ref=main
    ```

   Optionally, specify the version explicitly:

    ```bash
    gh workflow run publish-release.yml -f ref=main -f version=$VERSION
    ```

4. Verify the release appears on the
   [GitHub releases page](https://github.com/abhinav/doc2go/releases).
