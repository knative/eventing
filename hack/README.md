# Assorted scripts for development

This directory contains several scripts useful in the development process of
Knative Eventing.

- `boilerplate/add-boilerplate.sh` Adds license boilerplate to _txt_ or _go_
  files in a directory, recursively.
- `release.sh` Creates a new [release](release.md) of Knative Eventing.
- `update-codegen.sh` Updates auto-generated client libraries.
- `update-deps.sh` Updates Go dependencies.
- `verify-codegen.sh` Verifies that auto-generated client libraries are
  up-to-date.

## Creating a release

_Note: only Knative leads can create versioned releases._

See https://github.com/knative/test-infra/blob/master/ci for more information
on creating releases.

### Creating a major version release

1. Create and push a `release-X.Y` branch from `master`. You can use the git
   cli or the GitHub UI to create the branch. _You must have write
   permissions to the repo to create a branch._

Prow will detect the new release branch and run the `release.sh` script. If
the build succeeds, a new tag `vX.Y.0` will be created and a [GitHub release
published](https://github.com/knative/eventing/releases). If the build fails,
logs can be retrieved from https://testgrid.knative.dev/eventing#auto-release.

The major version release job is currently called
`ci-knative-eventing-auto-release` and runs
[every alternate hour](https://github.com/knative/test-infra/blob/957032b0badbf4409384995f3c34350f24f5f5ae/ci/prow/config.yaml).

### Creating a minor version release

1. Create a branch from the desired `release-X.Y` branch.
1. Cherry-pick commits into the new branch.
1. Create a PR with the new branch and `release-X.Y` as base.
1. Merge the PR.

Prow will detect the new commits in the release branch and run the `release.sh`
script. If the build succeeds, a new tag `vX.Y.Z` will be created (where `Z` is
the current minor version number + 1) and a
[GitHub release published](https://github.com/knative/eventing/releases). If the
build fails, logs can be retrieved from
https://testgrid.knative.dev/eventing#dot-release.

The minor version release job is currently called
`ci-knative-eventing-dot-release` and runs
[every Tuesday at 09:56 Pacific time](https://github.com/knative/test-infra/blob/957032b0badbf4409384995f3c34350f24f5f5ae/ci/prow/config.yaml#L2209).
