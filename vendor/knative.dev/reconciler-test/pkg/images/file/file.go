/*
Copyright 2023 The Knative Authors

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

package file

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ImageProducer is a file-based image producer.
// The expected file format is a YAML file like the following:
// <go-package-ref>: <image-ref>
//
// For example:
//
// knative.dev/reconciler-test/cmd/eventshub: quay.io/myregistry/eventshub
// # ... other images ...
func ImageProducer(filepath string) func(ctx context.Context, pack string) (string, error) {
	images := make(map[string]string, 2)

	bytes, err := os.ReadFile(filepath)
	if err != nil {
		panic("failed to read file " + filepath + ": " + err.Error())
	}

	if err := yaml.Unmarshal(bytes, images); err != nil {
		panic("failed to unmarshal images file: " + err.Error() + "\nContent:\n" + string(bytes))
	}

	return func(ctx context.Context, pack string) (string, error) {
		im, ok := images[pack]
		if !ok {
			return "", fmt.Errorf("image for package %s not found in %+v", pack, images)
		}
		return im, nil
	}
}
