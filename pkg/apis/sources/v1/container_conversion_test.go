/*
Copyright 2020 The Knative Authors

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

package v1

import (
	"context"
	"testing"
)

func TestContainerSourceConversionBadType(t *testing.T) {
	good, bad := &ContainerSource{}, &testObject{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Error("ConvertTo() = nil, wanted error")
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Error("ConvertFrom() = nil, wanted error")
	}
}
