# Release process

> This document is intended for doc2go maintainers only.

## Prerequisites

The following must be installed for a new release:

- [GitHub CLI (gh)](https://cli.github.com/)
- [changie](https://changie.dev/)

## Steps

To release a new version of doc2go, take the following steps:

1. Set an environment variable `VERSION` specifying the release version
   **with** the 'v' prefix.
   Be aware that doc2go follows [semver](https://semver.org/).

    ```bash
    VERSION=vX.Y.Z
    ```

2. Create a branch to prepare the release off `main`.

    ```bash
    git checkout main
    git pull
    git checkout -b prepare-$VERSION
    ```

3. Prepare the release notes for the new version.

    ```bash
    changie batch $VERSION
    ```

    Edit the generated file manually if desired.

4. Merge the release notes into the CHANGELOG.md.

    ```bash
    changie merge
    ```

5. Stage and commit everything.

    ```bash
    git add .changes CHANGELOG.md
    git commit -m "Prepare release $VERSION"
    ```

6. Create a pull request against the release branch.

    ```bash
    gh pr create -B release -t "Release $VERSION" -b ""
    ```

7. Once the build is green, merge the branch.

    ```bash
    gh pr merge -m -d
    ```

8. Tag and push the release.

    ```bash
    git tag -a "$VERSION" -m "$VERSION"
    git push origin $VERSION
    ```

9. Update main.

    ```bash
    git checkout main
    git merge origin/release
    git push
    ```
