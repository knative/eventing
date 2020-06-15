#!/bin/bash

# Usage: create-release-branch.sh v0.4.1 release-0.4

set -e # Exit immediately on error.

release=$1
target=$2

# Fetch the latest tags and checkout a new branch from the wanted tag.
git fetch upstream --tags
git checkout -b "$target" "$release"

# Copy the openshift extra files from the OPENSHIFT/master branch.
git fetch openshift master
git checkout openshift/master -- openshift OWNERS_ALIASES OWNERS Makefile
make generate-dockerfiles
make RELEASE=$release generate-release
git add openshift OWNERS_ALIASES OWNERS Makefile
git commit -m "Add openshift specific files."
