package broker

import (
	"fmt"
	v1 "knative.dev/pkg/apis/duck/v1"

	rektr "knative.dev/eventing/test/rekt/resources"
)

// The following examples validate the processing of the With* helper methods
// applied to config and go template parser.

func Example_min() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
		"subscriber": map[string]interface{}{
			"ref": map[string]string{
				"kind":       "subkind",
				"name":       "subname",
				"apiVersion": "subversion",
			},
		},
	}

	files, err := rektr.ParseLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	more := false
	for _, file := range files {
		if more {
			fmt.Println("---")
		}
		more = true
		yaml := rektr.RemoveBlanks(rektr.RemoveComments(string(file)))
		fmt.Println(yaml)
	}

	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   broker: baz
	//   subscriber:
	//     ref:
	//       kind: subkind
	//       namespace: bar
	//       name: subname
	//       apiVersion: subversion
}

func Example_full() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
		"filter": map[string]interface{}{
			"attributes": map[string]string{
				"x":    "y",
				"type": "z",
			},
		},
		"subscriber": map[string]interface{}{
			"ref": map[string]string{
				"kind":       "subkind",
				"name":       "subname",
				"apiVersion": "subversion",
			},
			"uri": "/extra/path",
		},
	}

	files, err := rektr.ParseLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	rektr.OutputYAML(files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   broker: baz
	//   filter:
	//     attributes:
	//       type: z
	//       x: y
	//   subscriber:
	//     ref:
	//       kind: subkind
	//       namespace: bar
	//       name: subname
	//       apiVersion: subversion
	//     uri: /extra/path
}

func ExampleWithSubscriber() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
	}

	WithSubscriber(&v1.KReference{
		Kind:       "subkind",
		Name:       "subname",
		APIVersion: "subversion",
	}, "/extra/path")(cfg)

	files, err := rektr.ParseLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	rektr.OutputYAML(files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   broker: baz
	//   subscriber:
	//     ref:
	//       kind: subkind
	//       namespace: bar
	//       name: subname
	//       apiVersion: subversion
	//     uri: /extra/path
}

func ExampleWithFilter() {
	images := map[string]string{}
	cfg := map[string]interface{}{
		"name":       "foo",
		"namespace":  "bar",
		"brokerName": "baz",
	}

	WithFilter(map[string]string{
		"x":    "y",
		"type": "z",
	})(cfg)

	files, err := rektr.ParseLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	rektr.OutputYAML(files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: foo
	//   namespace: bar
	// spec:
	//   broker: baz
	//   filter:
	//     attributes:
	//       type: z
	//       x: y
}
