#!/usr/bin/env bash 

set -e

ROOT_DIR=$(dirname $0)/..

action="$1"

function usage() {
	cmd="$1"

	echo ""
	echo "Usage                                                for more information read DEVELOPMENT.md"
	echo "$cmd <command>"
	echo ""
	echo "command:"
	echo "   install                                           Install Knative components for local dev"
	echo "   e2e-debug <test-name> <test-dir>                  Quick debug"
	echo "   update-cert-manager                               Update Cert manager"
	echo "   generate-yamls <repo-root-dir> <output-file>      Generate all repo yamls "
	echo "   update-reference-docs                             Update reference docs"
	echo "   teardown                                          Teardown installed components for local dev"
	echo "   update-checksums                                  Update checksums"
	echo "   update-codegen                                    Update codegen"
	echo "   update-deps                                       Update deps"
	echo "   verify-codegen                                    Verify codegen"
	echo ""
}

if [[ "${action}" == "install" ]]; then
	./"${ROOT_DIR}"/hack/install.sh 
elif [[ "${action}" == "update-cert-manager" ]]; then
	./"${ROOT_DIR}"/hack/update-cert-manager.sh "$2" "$3"
elif [[ "${action}" == "e2e-debug" ]]; then
	./"${ROOT_DIR}"/hack/e2e-debug.sh "$2" "$3"
elif [[ "${action}" == "generate-yamls" ]]; then
	./"${ROOT_DIR}"/hack/generate-yamls.sh "$2" "$3"
elif [[ "${action}" == "update-reference-docs" ]]; then
	./"${ROOT_DIR}"/hack/update-reference-docs.sh
elif [[ "${action}" == "teardown" ]]; then
	./"${ROOT_DIR}"/hack/teardown.sh
elif [[ "${action}" == "update-checksums" ]]; then
	./"${ROOT_DIR}"/hack/update-checksums.sh
elif [[ "${action}" == "update-codegen" ]]; then
	./"${ROOT_DIR}"/hack/update-codegen.sh
elif [[ "${action}" == "update-deps" ]]; then
	./"${ROOT_DIR}"/hack/update-deps.sh 
elif [[ "${action}" == "verify-codegen" ]]; then
	./"${ROOT_DIR}"/hack/verify-codegen.sh
else
	echo "Unrecognized action ${action}"
	usage "$0"
	exit 1
fi