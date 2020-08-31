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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Brokers holds the Brokers we want to run test against.
type Brokers []metav1.TypeMeta

func (brokers *Brokers) String() string {
	return fmt.Sprint(*brokers)
}

// Set appends the input string to Brokers.
func (brokers *Brokers) Set(value string) error {
	*brokers = csvToObjects(value, isValidBroker)
	return nil
}

// Check if the broker kind is valid.
func isValidBroker(broker string) bool {
	return strings.HasSuffix(broker, "Broker")
}
