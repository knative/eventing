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

package tracing

import (
	"context"
	"reflect"
	"testing"
)

func TestSpanData(t *testing.T) {
	ctx := context.Background()
	sd := SpanDataFromContext(ctx)
	if sd != nil {
		t.Errorf("SpanDataFromContext() is %v, wanted nil", sd)
	}

	want := SpanData{
		Name:       "name",
		Kind:       0,
		Attributes: nil,
	}

	ctx = WithSpanData(ctx, "name", 0, nil)
	sd = SpanDataFromContext(ctx)

	if !reflect.DeepEqual(sd, &want) {
		t.Errorf("SpanDataFromContext(). got %v, wanted %v", sd, want)
	}
}
