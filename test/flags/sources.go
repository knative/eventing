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
	"fmt"
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Sources holds the Sourcess we want to run test against.
type Sources []metav1.TypeMeta

func (sources *Sources) String() string {
	return fmt.Sprint(*sources)
}

// Set converts the input string to Sources.
func (sources *Sources) Set(value string) error {
	for _, source := range strings.Split(value, ",") {
		source := strings.TrimSpace(source)
		split := strings.Split(source, ":")
		if len(split) != 2 {
			log.Fatalf("The given Source name %q is invalid, it needs to be in the form \"apiVersion:Kind\".", source)
		}
		tm := metav1.TypeMeta{
			APIVersion: split[0],
			Kind:       split[1],
		}
		if !isValidSource(tm.Kind) {
			log.Fatalf("The given source name %q is invalid, tests cannot be run.\n", source)
		}

		*sources = append(*sources, tm)
	}
	return nil
}

// Check if the Source name is valid.
func isValidSource(source string) bool {
	return strings.HasSuffix(source, "Source")
}
