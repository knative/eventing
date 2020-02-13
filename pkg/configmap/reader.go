/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package configmap

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

// ReadIntRequest specifies the key to read from a configmap. Its value is set to the Field.
type ReadIntRequest struct {
	// Key in the configmap to read.
	Key string
	// Field is the int field to set.
	Field *int
	// DefaultValue is the default value to use if Key doesn't exist.
	DefaultValue int
}

// ReadInt reads int values from a given configmap according to the specs.
func ReadInt(requests []ReadIntRequest, config *corev1.ConfigMap) error {
	data := config.Data
	for _, request := range requests {
		if raw, ok := data[request.Key]; !ok {
			*request.Field = request.DefaultValue
		} else if val, err := strconv.Atoi(raw); err != nil {
			return err
		} else {
			*request.Field = val
		}
	}
	return nil
}
