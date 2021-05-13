// +build hackhackhack

/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rekt

import (
	_ "knative.dev/pkg/injection"
)

/*
HACK HACK HACK

the pkg package "knative.dev/pkg/injection" fixes a workaround for working with
Knative injection and testing issues with

$ go test -vet=off -tags "${tags}" --count=1 -run=^$ ./...

But we need to test with the the hackhackhack tag, which is collected by
searching the repo for files that have tags. This file enables the pkg
file to work.
*/
