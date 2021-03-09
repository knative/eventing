# Assorted scripts for development

This directory contains several scripts useful in the development process of
Knative Eventing.

- `boilerplate/add-boilerplate.sh` Adds license boilerplate to _txt_ or _go_
  files in a directory, recursively.
- `release.sh` Creates a new [release](#creating-a-release) of Knative Eventing.
- `update-codegen.sh` Updates auto-generated client libraries.
- `update-deps.sh` Updates Go dependencies.
- `verify-codegen.sh` Verifies that auto-generated client libraries are
  up-to-date.

## Creating a release

_Note: only Knative leads can create versioned releases._

See https://github.com/knative/test-infra/blob/main/ci for more information on
creating releases.

### Creating a major version release

#### GitHub UI

1. Click the Branch dropdown.
1. Type the desired `release-X.Y` branch name into the search box.
1. Click the `Create branch: release-X.Y from 'master'` button. _You must have
   write permissions to the repo to create a branch._

   Prow will detect the new release branch and run the `release.sh` script. If
   the build succeeds, a new tag `vX.Y.0` will be created and a GitHub release
   published. If the build fails, logs can be retrieved from
   https://testgrid.knative.dev/eventing#auto-release.

1. Write release notes and add them to
   [the release](https://github.com/knative/eventing/releases).

#### Git CLI

1.  Fetch the upstream remote.

    ```sh
    git fetch upstream
    ```

1.  Create a `release-X.Y` branch from `upstream/master`.

    ```sh
    git branch --no-track release-X.Y upstream/master
    ```

1.  Push the branch to upstream.

    ```sh
    git push upstream release-X.Y
    ```

    _You must have write permissions to the repo to create a branch._

    Prow will detect the new release branch and run the `release.sh` script. If
    the build succeeds, a new tag `vX.Y.0` will be created and a GitHub release
    published. If the build fails, logs can be retrieved from
    https://testgrid.knative.dev/eventing#auto-release.

1.  Write release notes and add them to
    [the release](https://github.com/knative/eventing/releases).

The major version release job is currently called
`ci-knative-eventing-auto-release` and runs
[every alternate hour](https://github.com/knative/test-infra/blob/957032b0badbf4409384995f3c34350f24f5f5ae/ci/prow/config.yaml).

### Creating a minor version release

1.  Fetch the upstream remote.

    ```sh
    git fetch upstream
    ```

1.  Create a branch based on the desired `release-X.Y` branch.

    ```sh
    git co -b my-backport-branch upstream/release-X.Y
    ```

1.  Cherry-pick desired commits from master into the new branch.

    ```sh
    git cherry-pick <commitid>
    ```

1.  Push the branch to your fork.

    ```sh
    git push origin
    ```

1.  Create a PR for your branch based on the `release-X.Y` branch.

    Once the PR is merged, Prow will detect the new commits in the release
    branch and run the `release.sh` script. If the build succeeds, a new tag
    `vX.Y.Z` will be created (where `Z` is the current minor version number + 1)
    and a
    [GitHub release published](https://github.com/knative/eventing/releases). If
    the build fails, logs can be retrieved from
    https://testgrid.knative.dev/eventing#dot-release.

1.  Write release notes and add them to
    [the release](https://github.com/knative/eventing/releases).

The minor version release job is currently called
`ci-knative-eventing-dot-release` and runs
[every Tuesday at 09:56 Pacific time](https://github.com/knative/test-infra/blob/957032b0badbf4409384995f3c34350f24f5f5ae/ci/prow/config.yaml#L2209).
