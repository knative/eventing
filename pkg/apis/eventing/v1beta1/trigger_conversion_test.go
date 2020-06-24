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

package v1beta1

import (
	"context"
	"testing"

	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

func TestTriggerConversion(t *testing.T) {
	objv1betav1, objv1 := &Trigger{}, &v1.Trigger{}

	if err := objv1betav1.ConvertTo(context.Background(), objv1); err != nil {
		t.Errorf("ConvertTo() = %#v, unexpected error", objv1betav1)
	}

	if err := objv1betav1.ConvertFrom(context.Background(), objv1); err != nil {
		t.Errorf("ConvertFrom() = %#v, unexpected error", objv1)
	}
}

func TestTriggerConversionBadType(t *testing.T) {
	good, bad := &Trigger{}, &Broker{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestTriggerConversionBadVersion(t *testing.T) {
	good, bad := &Trigger{}, &Trigger{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}
