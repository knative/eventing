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

// Channels holds the Channels we want to run test against.
type Channels []metav1.TypeMeta

func (channels *Channels) String() string {
	return fmt.Sprint(*channels)
}

// Set appends the input string to Channels.
func (channels *Channels) Set(value string) error {
	*channels = csvToObjects(value, isValidChannel)
	return nil
}

// Check if the channel kind is valid.
func isValidChannel(channel string) bool {
	return strings.HasSuffix(channel, "Channel")
}
