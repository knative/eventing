/*
Copyright 2018 The Knative Authors

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

package channel

import (
	"fmt"
	"net/url"
	"strings"
)

// ChannelReference references a Channel within the cluster by name and
// namespace.
type ChannelReference struct {
	Namespace string
	Name      string
}

func (r *ChannelReference) String() string {
	return fmt.Sprintf("%s/%s", r.Namespace, r.Name)
}

// ParseChannel determines a Channel reference from a URL
func ParseChannel(rawURL string) (ChannelReference, error) {
	url, err := url.Parse(rawURL)
	if err != nil {
		return ChannelReference{}, fmt.Errorf("bad url: %s", rawURL)
	}

	path := url.Path
	if path == "/" {
		// host based routing
		host := url.Host
		chunks := strings.Split(host, ".")
		if len(chunks) < 2 {
			return ChannelReference{}, fmt.Errorf("bad host format %q", host)
		}
		return ChannelReference{
			Name:      chunks[0],
			Namespace: chunks[1],
		}, nil
	}

	// path based routing
	splitPath := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(splitPath) != 3 {
		return ChannelReference{}, fmt.Errorf("bad path format %s", path)
	}

	return ChannelReference{
		Namespace: splitPath[1],
		Name:      splitPath[2],
	}, nil
}
