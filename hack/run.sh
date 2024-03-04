#!/usr/bin/env bash 

set -e

ROOT_DIR=$(dirname $0)/..

action="$1"

function usage() {
	cmd="$1"

	echo ""
	echo "Usage                                                      for more information read DEVELOPMENT.md"
	echo "$cmd <command>"
	echo ""
	echo "command:"
	echo "   create-kind-cluster                                     Create a new kind cluster"
	echo "   install                                         				 Quick full build and install"
	echo "   e2e-debug <test-name> <test-dir>                  			 Quick debug"
	echo "   update-cert-manager <cert-manager> <trust-manager>      Update manager"
	echo "   generate-yamls <repo-root-dir> <output-file>            Generate all repo yamls "
	echo "   update-reference-docs                                   Update reference docs"
	echo "   release                                                 Release new version"
	echo "   teardown                                        				 Clean up and uninstall"
	echo "   update-checksums                                        Update checksums"
	echo "   update-codegen              													 	 Update codegen"
	echo "   update-deps                    												 Update deps"
	echo "   verify-codegen                                          Verify codegen"
	echo ""
}

if [[ "$action" == "create-kind-cluster" ]]; then
	source "${ROOT_DIR}"/hack/create-kind-cluster.sh 
elif [[ "${action}" == "install" ]]; then
	source "${ROOT_DIR}"/hack/install.sh 
elif [[ "${action}" == "update-cert-manager" ]]; then
	source "${ROOT_DIR}"/hack/update-cert-manager.sh "$2" "$3"
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
elif [[ "${action}" == "update-checksums" ]]; then
	source "${ROOT_DIR}"/hack/update-checksums.sh
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