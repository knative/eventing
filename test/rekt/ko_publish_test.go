//go:build e2e
// +build e2e

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

package rekt

import (
	"fmt"
	"os"
	"testing"
	"text/template"

	_ "knative.dev/pkg/system/testing"
	"knative.dev/reconciler-test/pkg/environment"
)

// This test is more for debugging the ko publish process.
func TestKoPublish(t *testing.T) {
	ic, err := environment.ProduceImages()
	if err != nil {
		panic(fmt.Errorf("failed to produce images, %s", err))
	}

	templateString := `
// The following could be used to bypass the image generation process.

import "knative.dev/reconciler-test/pkg/environment"

func init() {
	environment.WithImages(map[string]string{
		{{ range $key, $value := . }}"{{ $key }}": "{{ $value }}",
		{{ end }}
	})
}
`

	tp := template.New("t")
	temp, err := tp.Parse(templateString)
	if err != nil {
		panic(err)
	}

	err = temp.Execute(os.Stdout, ic)
	if err != nil {
		panic(err)
	}
	_, _ = fmt.Fprint(os.Stdout, "\n\n")
}
