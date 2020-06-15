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

// Channels holds the Channels we want to run test against.
type Channels []metav1.TypeMeta

func (channels *Channels) String() string {
	return fmt.Sprint(*channels)
}

// Set appends the input string to Channels.
func (channels *Channels) Set(value string) error {
	for _, channel := range strings.Split(value, ",") {
		channel := strings.TrimSpace(channel)
		split := strings.Split(channel, ":")
		if len(split) != 2 {
			log.Fatalf("The given Channel name %q is invalid, it needs to be in the form \"apiVersion:Kind\".", channel)
		}
		tm := metav1.TypeMeta{
			APIVersion: split[0],
			Kind:       split[1],
		}
		if !isValidChannel(tm.Kind) {
			log.Fatalf("The given Channel name %q is invalid, tests cannot be run.\n", channel)
		}

		*channels = append(*channels, tm)
	}
	return nil
}

// Check if the channel name is valid.
func isValidChannel(channel string) bool {
	return strings.HasSuffix(channel, "Channel")
}
