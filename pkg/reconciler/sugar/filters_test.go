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

package sugar

import (
	"context"
	"os"
	"reflect"
	"runtime"
	"testing"
)

func TestOnByDefault(t *testing.T) {
	testCases := map[string]struct {
		given map[string]string
		want  bool
	}{
		"nil": {
			want: true,
		},
		"empty": {
			given: map[string]string{},
			want:  true,
		},
		"other": {
			given: map[string]string{
				"unrelated": "values",
			},
			want: true,
		},
		"labeled, enabled": {
			given: map[string]string{
				InjectionLabelKey: InjectionEnabledLabelValue,
			},
			want: true,
		},
		"labeled, disabled": {
			given: map[string]string{
				InjectionLabelKey: InjectionDisabledLabelValue,
			},
			want: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			got := OnByDefault(tc.given)
			if got != tc.want {
				t.Errorf("Expected: %t but got: %t", tc.want, got)
			}
		})
	}
}

func TestOffByDefault(t *testing.T) {
	testCases := map[string]struct {
		given map[string]string
		want  bool
	}{
		"nil": {
			want: false,
		},
		"empty": {
			given: map[string]string{},
			want:  false,
		},
		"other": {
			given: map[string]string{
				"unrelated": "values",
			},
			want: false,
		},
		"labeled, enabled": {
			given: map[string]string{
				InjectionLabelKey: InjectionEnabledLabelValue,
			},
			want: true,
		},
		"labeled, disabled": {
			given: map[string]string{
				InjectionLabelKey: InjectionDisabledLabelValue,
			},
			want: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			got := OffByDefault(tc.given)
			if got != tc.want {
				t.Errorf("Expected: %t but got: %t", tc.want, got)
			}
		})
	}
}

func TestLabelFilterFnOrDieInjectionOn(t *testing.T) {
	ctx := context.Background()
	os.Setenv("BROKER_INJECTION_DEFAULT", "true")
	want := OnByDefault
	if got := LabelFilterFnOrDie(ctx); !reflect.DeepEqual(reflect.ValueOf(got).Pointer(), reflect.ValueOf(want).Pointer()) {
		t.Errorf("LabelFilterFnOrDie() = %v, want %v", getFunctionName(got), getFunctionName(want))
	}
	os.Unsetenv("BROKER_INJECTION_DEFAULT")

}

func TestLabelFilterFnOrDieInjectionOff(t *testing.T) {
	ctx := context.Background()
	want := OffByDefault
	if got := LabelFilterFnOrDie(ctx); !reflect.DeepEqual(reflect.ValueOf(got).Pointer(), reflect.ValueOf(want).Pointer()) {
		t.Errorf("LabelFilterFnOrDie() = %v, want %v", getFunctionName(got), getFunctionName(want))
	}
	os.Setenv("BROKER_INJECTION_DEFAULT", "false")
	if got := LabelFilterFnOrDie(ctx); !reflect.DeepEqual(reflect.ValueOf(got).Pointer(), reflect.ValueOf(want).Pointer()) {
		t.Errorf("LabelFilterFnOrDie() = %v, want %v", getFunctionName(got), getFunctionName(want))
	}
	os.Unsetenv("BROKER_INJECTION_DEFAULT")

}
func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
