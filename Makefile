#This makefile is used by ci-operator

CGO_ENABLED=0
GOOS=linux
CORE_IMAGES=./cmd/controller/ ./cmd/webhook/ ./pkg/provisioners/kafka ./cmd/fanoutsidecar
TEST_IMAGES=./test/test_images/k8sevents

install:
	go install $(CORE_IMAGES)
	go build -o $(GOPATH)/bin/in-memory-channel-controller ./pkg/controller/eventing/inmemory/controller
.PHONY: install

test-install:
	go install $(TEST_IMAGES)
.PHONY: test-install

test-e2e:
	sh openshift/e2e-tests-openshift.sh
.PHONY: test-e2e

# Generate Dockerfiles used by ci-operator. The files need to be committed manually.
generate-dockerfiles:
	./openshift/ci-operator/generate-dockerfiles.sh openshift/ci-operator/knative-images $(CORE_IMAGES)
	./openshift/ci-operator/generate-dockerfiles.sh openshift/ci-operator/knative-images in-memory-channel-controller
	./openshift/ci-operator/generate-dockerfiles.sh openshift/ci-operator/knative-test-images $(TEST_IMAGES)
.PHONY: generate-dockerfiles

# Generates a release.yaml for a specific branch.
generate-release:
	./openshift/ci-operator/generate-release.sh $(BRANCH) > release.yaml
.PHONY: generate-release
