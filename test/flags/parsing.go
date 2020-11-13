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

package flags

import (
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Validator func(string) bool

func csvToObjects(csvObjects string, validator Validator) []metav1.TypeMeta {
	result := make([]metav1.TypeMeta, 0, 5)
	for _, object := range strings.Split(csvObjects, ",") {
		object := strings.TrimSpace(object)
		split := strings.Split(object, ":")
		if len(split) != 2 {
			log.Fatalf(`The given object name %q is invalid, it needs to be in the form "apiVersion:Kind".`, object)
		}
		tm := metav1.TypeMeta{
			APIVersion: split[0],
			Kind:       split[1],
		}
		if !validator(tm.Kind) {
			log.Fatalf("The given object name %q is invalid, tests cannot be run.\n", object)
		}

		result = append(result, tm)
	}
	return result
}
