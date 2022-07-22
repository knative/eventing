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

package ko

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// ErrKoPublishFailed is returned when the ko publish command fails.
var ErrKoPublishFailed = errors.New("ko publish failed")

// Publish uses ko to publish the image.
func Publish(ctx context.Context, path string) (string, error) {
	version := os.Getenv("GOOGLE_KO_VERSION")
	if version == "" {
		version = "v0.11.2"
	}
	args := []string{
		"go", "run", fmt.Sprintf("github.com/google/ko@%s", version),
		"publish",
	}
	platform := os.Getenv("PLATFORM")
	if len(platform) > 0 {
		args = append(args, "--platform="+platform)
	}
	args = append(args, "-B", path)
	out, err := runCmd(ctx, args)
	if err != nil {
		return "", fmt.Errorf("%w: %v -- command: %q",
			ErrKoPublishFailed, err, args)
	}
	return out, nil
}
