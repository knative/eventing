#This makefile is used by ci-operator

CGO_ENABLED=0
GOOS=linux
CORE_IMAGES=$(shell find ./cmd -name main.go ! -path "./cmd/broker/*" ! -path "./cmd/mtbroker/*" | sed 's/main.go//')
TEST_IMAGES=$(shell find ./test/test_images -mindepth 1 -maxdepth 1 -type d)

# Guess location of openshift/release repo. NOTE: override this if it is not correct.
OPENSHIFT=${CURDIR}/../../github.com/openshift/release

install:
	go install $(CORE_IMAGES)
	go build -o $(GOPATH)/bin/mtbroker_ingress ./cmd/mtbroker/ingress/
	go build -o $(GOPATH)/bin/mtbroker_filter ./cmd/mtbroker/filter/
.PHONY: install

test-install:
	go install $(TEST_IMAGES)
.PHONY: test-install

test-e2e:
	sh openshift/e2e-tests-openshift.sh
.PHONY: test-e2e

test-origin-conformance:
	sh TEST_ORIGIN_CONFORMANCE=true openshift/e2e-tests-openshift.sh
.PHONY: test-origin-conformance

# Generate Dockerfiles used by ci-operator. The files need to be committed manually.
generate-dockerfiles:
	rm -rf openshift/ci-operator/knative-images/*
	./openshift/ci-operator/generate-dockerfiles.sh openshift/ci-operator/knative-images $(CORE_IMAGES)
	./openshift/ci-operator/generate-dockerfiles.sh openshift/ci-operator/knative-images mtbroker_ingress
	./openshift/ci-operator/generate-dockerfiles.sh openshift/ci-operator/knative-images mtbroker_filter
	rm -rf openshift/ci-operator/knative-test-images/*
	./openshift/ci-operator/generate-dockerfiles.sh openshift/ci-operator/knative-test-images $(TEST_IMAGES)
.PHONY: generate-dockerfiles

# Generate an aggregated knative release yaml file, as well as a CI file with replaced image references
generate-release:
	./openshift/release/generate-release.sh $(RELEASE)
.PHONY: generate-release

# Update CI configuration in the $(OPENSHIFT) directory.
# NOTE: Makes changes outside this repository.
update-ci:
	sh ./openshift/ci-operator/update-ci.sh $(OPENSHIFT) $(CORE_IMAGES)
.PHONY: update-ci
