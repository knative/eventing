# Release creation

**Note 1** Run all scripts from the root of the repository.

**Note 2** The master branch in this repo is used as a stash for
openshift-specific files needed for CI. Those files are copied to release
branches which is where CI operates.

## Setting up your clone

**Note** Your clone must be in `$GOPATH/src/knative.dev/eventing` *not*
`github.com/knative/eventing` or `openshift/knative-eventing`

You must have remotes named "upstream" and "openshift" for the scripts
in this repo to work, like this:

```
git remote add openshift git@github.com:openshift/knative-eventing.git
git remote add upstream git@github.com:knative/eventing.git
```

## Branching
**On the master branch** create a release branch and then push it upstream:

```bash
./openshift/release/create-release-branch.sh vX.Y.Z release-vX.Y.Z
git push -v openshift refs/heads/release-vX.Y.Z\:refs/heads/release-vX.Y.Z
```

This creates and checks out "release-vX.Y.Z" based on tag "vX.Y.Z" and adds
OpenShift specific files that we need to run CI.

All remaining steps must be done **on the release branch**.

## Building image and docker files

On the release branch, build the images and docker files.

```
make install && make generate-dockerfiles
```

If any are new/changed, check them in.

## Update CI configuration

To enable CI, you need to update files on the master branch of github.com/openshift/release.
This command will update the files, assuming that you have an openshift/release clone
in the same tree as this repository; if not add OPENSHIFT=<path-to-release>

```
make update-ci VERSION=X.Y.Z
```

This creates and modifies files in the openshift/release repo, verify that those files
are as expected, commit and create a PR for them. That will start a CI job.

## Update this README

If you find that any of the steps are incorrect or out of date.

# Updating a branch that follow upstream's HEAD

This is done via the nightly Jenkins job to create the release-next branch:

```bash
./openshift/release/update-to-head.sh release-vX.Y.Z
```

This pulls the latest master from upstream, rebase the current fixes on the
release-vX.Y.Z branch and updates the Openshift specific files if necessary.

