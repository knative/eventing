/*
Copyright 2024 The Knative Authors

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

package v1alpha1

import (
	"context"
	"errors"
	"testing"

	"knative.dev/pkg/apis"
)

// implement apis.Convertible
type testObject struct{}

func (*testObject) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	return errors.New("Won't go")
}

func (*testObject) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	return errors.New("Won't go")
}

func TestConversionConversionBadType(t *testing.T) {
	good, bad := &JobSink{}, &testObject{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}
