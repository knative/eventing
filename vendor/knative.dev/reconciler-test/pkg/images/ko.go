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
)

// Use ko to publish the image.
func KoPublish(path string) (string, error) {
	fmt.Println(fmt.Sprintf("ko publish -B %s", path))
	out, err := runCmd(fmt.Sprintf("ko publish -B %s", path))
	if err != nil {
		return "", err
	}
	return out, nil
}
