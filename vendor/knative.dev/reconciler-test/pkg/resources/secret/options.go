/*
 * Copyright 2023 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package secret

import (
	"encoding/base64"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/reconciler-test/pkg/manifest"
)

var WithAnnotations = manifest.WithAnnotations
var WithLabels = manifest.WithLabels

func WithData(data map[string][]byte) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if data != nil {
			base64Data := map[string]string{}
			for key, val := range data {
				base64Data[key] = base64.StdEncoding.EncodeToString(val)
			}

			cfg["data"] = base64Data
		}
	}
}

func WithStringData(stringdata map[string]string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if stringdata != nil {
			cfg["stringdata"] = stringdata
		}
	}
}

func WithType(secretType corev1.SecretType) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["type"] = secretType
	}
}
