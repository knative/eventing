#!/usr/bin/env bash

# Copyright 2021 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script creates test namespaces together with ServiceAccounts, Roles,
# RoleBindings for conformance tests. This script is useful when tests are
# run with --reusenamespace option in restricted environments. See README.md
# for more information.

set -Eeuo pipefail

NUM_NAMESPACES=${NUM_NAMESPACES:?"Pass the NUM_NAMESPACES env variable"}
EVENTING_E2E_NAMESPACE="${EVENTING_E2E_NAMESPACE:-eventing-e2e}"

for i in $(seq 0 "$(("$NUM_NAMESPACES" - 1))"); do
  cat <<EOF | sed "s/__NAME__/${EVENTING_E2E_NAMESPACE}${i}/" | kubectl apply -f -
---
apiVersion: v1
kind: Namespace
metadata:
  name: __NAME__
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: __NAME__
  namespace: __NAME__
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: __NAME__
  namespace: __NAME__
rules:
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - '*'
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: __NAME__
  namespace: __NAME__
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: __NAME__
subjects:
- kind: ServiceAccount
  name: __NAME__
  namespace: __NAME__
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: __NAME__-eventwatcher
  namespace: __NAME__
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: __NAME__-eventwatcher
  namespace: __NAME__
rules:
- apiGroups:
  - ""
  resources:
  - pods
  - events
  verbs:
  - get
  - list
  - watch
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: __NAME__-eventwatcher
  namespace: __NAME__
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: __NAME__-eventwatcher
subjects:
- kind: ServiceAccount
  name: __NAME__-eventwatcher
  namespace: __NAME__
EOF
done
