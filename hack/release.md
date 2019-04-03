# Creating a new Knative Eventing release

The `release.sh` script automates the creation of Knative Eventing releases,
either nightly or versioned ones.

By default, the script creates a nightly release but does not publish it
anywhere.

## Creating versioned releases

_Note: only Knative leads can create versioned releases._

Versioned releases are automatically created by Prow.

### Creating a major version release

1. Create and push a `release-X.Y` branch from `master`. You can use the git cli
   or the GitHub UI to create the branch. _You must have write permissions to
   the repo to create a branch._

Prow will detect the new release branch and run the `release.sh` script. If the
build succeeds, a new tag `vX.Y.0` will be created and a GitHub release
published.

The major version release job currently runs
[5 minutes after the hour, every 2 hours](https://github.com/knative/test-infra/blob/957032b0badbf4409384995f3c34350f24f5f5ae/ci/prow/config.yaml#L2251).

### Creating a minor version release

1. Create a branch from the desired `release-X.Y` branch.
1. Cherry-pick commits into the new branch.
1. Create a PR with the new branch and `release-X.Y` as base.
1. Merge the PR.

Prow will detect the new commits in the release branch and run the `release.sh`
script. If the build succeeds, a new tag `vX.Y.Z` will be created (where `Z` is
the current minor version number + 1) and a GitHub release published.

The minor version release job currently runs
[every Tuesday at 09:19 Pacific time](https://github.com/knative/test-infra/blob/957032b0badbf4409384995f3c34350f24f5f5ae/ci/prow/config.yaml#L2209).

### Creating a versioned release manually

To specify a versioned release to be cut, you must use the `--version` flag.
Versioned releases are usually built against a branch in the Knative Eventing
repository, specified by the `--branch` flag.

- `--version` Defines the version of the release, and must be in the form
  `X.Y.Z`, where X, Y and Z are numbers.
- `--branch` Defines the branch in Knative Eventing repository from which the
  release will be built. If not passed, the `master` branch at HEAD will be
  used. This branch must be created before the script is executed, and must be
  in the form `release-X.Y`, where X and Y must match the numbers used in the
  version passed in the `--version` flag. This flag has no effect unless
  `--version` is also passed.
- `--release-notes` Points to a markdown file containing a description of the
  release. This is optional but highly recommended. It has no effect unless
  `--version` is also passed.

If this is the first time you're cutting a versioned release, you'll be prompted
for your GitHub username, password, and possibly 2-factor authentication
challenge before the release is published.

The release will be published in the _Releases_ page of the Knative Eventing
repository, with the title _Knative Eventing release vX.Y.Z_ and the given
release notes. It will also be tagged _vX.Y.Z_ (both on GitHub and as a git
annotated tag).

## Creating nightly releases

Nightly releases are built against the current git tree. The behavior of the
script is defined by the common flags. You must have write access to the GCR and
GCS bucket the release will be pushed to, unless `--nopublish` is used.

Examples:

```bash
# Create and publish a nightly, tagged release.
./hack/release.sh --publish --tag-release

# Create release, but don't test, publish or tag it.
./hack/release.sh --skip-tests --nopublish --notag-release
```

## Common flags for cutting releases

The following flags affect the behavior of the script, no matter the type of the
release.

- `--skip-tests` Do not run tests before building the release. Otherwise, build,
  unit and end-to-end tests are run and they all must pass for the release to be
  built.
- `--tag-release`, `--notag-release` Tag (or not) the generated images with
  either `vYYYYMMDD-<commit_short_hash>` (for nightly releases) or `vX.Y.Z` for
  versioned releases. These are docker tags. _For versioned releases, a tag is
  always added._
- `--release-gcs` Defines the GCS bucket where the manifests will be stored. By
  default, this is `knative-nightly/eventing`. This flag is ignored if the
  release is not being published.
- `--release-gcr` Defines the GCR where the images will be stored. By default,
  this is `gcr.io/knative-nightly`. This flag is ignored if the release is not
  being published.
- `--publish`, `--nopublish` Whether the generated images should be published to
  a GCR, and the generated manifests written to a GCS bucket or not. If yes, the
  `--release-gcs` and `--release-gcr` flags can be used to change the default
  values of the GCR/GCS used. If no, the images will be pushed to the `ko.local`
  registry, and the manifests written to the local disk only (in the repository
  root directory).
