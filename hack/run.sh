#!/usr/bin/env bash 

set -e

ROOT_DIR=$(dirname $0)/..

action="$1"

if [[ "$action" == "create-kind-cluster" ]]; then
	source "${ROOT_DIR}"/hack/create-kind-cluster.sh 
elif [[ "${action}" == "install" ]]; then
	source "${ROOT_DIR}"/hack/install.sh 
elif [[ "${action}" == "update-cert-manager" ]]; then
	source "${ROOT_DIR}"/hack/update-cert-manager.sh
elif [[ "${action}" == "e2e-debug" ]]; then
	source "${ROOT_DIR}"/hack/e2e-debug.sh "$2" "$3"
elif [[ "${action}" == "generate-yamls" ]]; then
	source "${ROOT_DIR}"/hack/generate-yamls.sh "$2" "$3"
elif [[ "${action}" == "update-reference-docs" ]]; then
	source "${ROOT_DIR}"/hack/update-reference-docs.sh
elif [[ "${action}" == "release" ]]; then
	source "${ROOT_DIR}"/hack/release.sh
elif [[ "${action}" == "teardown" ]]; then
	source "${ROOT_DIR}"/hack/teardown.sh
elif [[ "${action}" == "update-check-sums" ]]; then
	source "${ROOT_DIR}"/hack/update-check-sums.sh
elif [[ "${action}" == "update-codegen" ]]; then
	source "${ROOT_DIR}"/hack/update-codegen.sh
elif [[ "${action}" == "update-deps" ]]; then
	source "${ROOT_DIR}"/hack/update-deps.sh 
elif [[ "${action}" == "verify-codegen" ]]; then
	source "${ROOT_DIR}"/hack/verify-codegen.sh
else
	echo "Unrecognized action ${action}"
	usage "$0"
	exit 1
fi