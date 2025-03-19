/*
 Copyright 2022 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package event

import (
	"gopkg.in/yaml.v2"
)

func ToYaml(event Event) ([]byte, error) {
	return yaml.Marshal(event)
}
