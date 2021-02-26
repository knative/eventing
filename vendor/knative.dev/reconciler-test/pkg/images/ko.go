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

package images

import (
	"fmt"
	"os"
)

// Use ko to publish the image.
func KoPublish(path string) (string, error) {
	platform := os.Getenv("PLATFORM")
	if len(platform) > 0 {
		platform = " --platform=" + platform
	}
	out, err := runCmd(fmt.Sprintf("ko publish%s -B %s", platform, path))
	if err != nil {
		return "", err
	}
	return out, nil
}
