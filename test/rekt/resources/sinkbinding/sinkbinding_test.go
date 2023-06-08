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

package sinkbinding_test

import (
	"embed"
	"os"

	testlog "knative.dev/reconciler-test/pkg/logging"
	"knative.dev/reconciler-test/pkg/manifest"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/eventing/test/rekt/resources/sinkbinding"
	"knative.dev/pkg/apis"
)

//go:embed *.yaml
var yaml embed.FS

// The following examples validate the processing of the With* helper methods
// applied to config and go template parser.

func Example_min() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"sink": map[string]interface{}{
			"ref": map[string]interface{}{
				"kind":       "AKind",
				"apiVersion": "something.valid/v1",
				"name":       "thesink",
			},
		},
		"subject": map[string]interface{}{
			"kind":       "BKind",
			"apiVersion": "interesting/v1",
			"name":       "thesubject",
		},
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: sources.knative.dev/v1
	// kind: SinkBinding
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   sink:
	//     ref:
	//       apiVersion: something.valid/v1
	//       kind: AKind
	//       namespace: bar
	//       name: thesink
	//   subject:
	//     kind: BKind
	//     apiVersion: interesting/v1
	//     namespace: bar
	//     name: thesubject
}

func Example_full() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
		"ceOverrides": map[string]interface{}{
			"extensions": map[string]string{
				"ext1": "val1",
				"ext2": "val2",
			},
		},
		"sink": map[string]interface{}{
			"ref": map[string]string{
				"kind":       "AKind",
				"name":       "thesink",
				"apiVersion": "something.valid/v1",
			},
			"uri": "uri/parts",
		},
		"subject": map[string]interface{}{
			"kind":       "BKind",
			"apiVersion": "interesting/v1",
			"name":       "thesubject",
			"selectorMatchLabels": map[string]string{
				"match1": "this",
				"match2": "that",
			},
		},
	}

	files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: sources.knative.dev/v1
	// kind: SinkBinding
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   ceOverrides:
	//     extensions:
	//       ext1: val1
	//       ext2: val2
	//   sink:
	//     ref:
	//       apiVersion: something.valid/v1
	//       kind: AKind
	//       namespace: bar
	//       name: thesink
	//     uri: uri/parts
	//   subject:
	//     kind: BKind
	//     apiVersion: interesting/v1
	//     namespace: bar
	//     name: thesubject
	//     selector:
	//       matchLabels:
	//         match1: this
	//         match2: that
}

func Example_withSink() {
	ctx := testlog.NewContext()
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":      "foo",
		"namespace": "bar",
	}

	sinkRef := &duckv1.Destination{
		Ref: &duckv1.KReference{
			Kind:       "sinkkind",
			Namespace:  "sinknamespace",
			Name:       "sinkname",
			APIVersion: "sinkversion",
		},
		URI: &apis.URL{Path: "uri/parts"},
         }
         sinkbinding.WithSink(sinkRef)(cfg)
	    files, err := manifest.ExecuteYAML(ctx, yaml, images, cfg)
	    if err != nil {
		panic(err)
	    }

	manifest.OutputYAML(os.Stdout, files)
        // Output:
        // apiVersion: sources.knative.dev/v1
       	// kind: SinkBinding
       	// metadata:
      	//   name: foo
      	//   namespace: bar
      	// spec:
      	//   sink:
      	//     ref:
      	//       apiVersion: sinkversion
      	//       kind: sinkkind
      	//       namespace: bar
      	//       name: sinkname
      	//     uri: uri/parts
     	//   subject:
    	//     kind: <no value>
   	//     apiVersion: <no value>
  	//     namespace: bar
}	
	
