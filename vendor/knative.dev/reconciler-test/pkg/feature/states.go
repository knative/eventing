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

package feature

import (
	"fmt"
	"strings"
)

type States uint8

const (
	// Alpha implies a feature is experimental and may change
	Alpha States = 1 << iota

	// Beta implies a feature is complete but may have unknown bugs or issues
	Beta

	// Stable implies a feature is ready for production
	Stable

	// Any flag enables any feature states
	Any = Alpha | Beta | Stable
)

func (s States) String() string {
	if s == 0 {
		return "None"
	}
	if s == Any {
		return "Any"
	}

	var b strings.Builder

	for state, name := range StatesMapping {
		if s&state == 0 {
			continue
		}

		if b.Len() != 0 {
			b.WriteString("|")
		}
		b.WriteString(name)
	}

	return b.String()
}

func (s States) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", s.String())), nil
}

var StatesMapping = map[States]string{
	Alpha:  "Alpha",
	Beta:   "Beta",
	Stable: "Stable",
}
