/*
 * Copyright 2018 The Knative Authors
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

package channel

import (
	"fmt"
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

// ParseChannel converts the channel's hostname into a channel
// reference.
func ParseChannel(host string) (ChannelReference, error) {
	chunks := strings.Split(host, ".")
	if len(chunks) < 2 {
		return ChannelReference{}, fmt.Errorf("bad host format %q", host)
	}
	return ChannelReference{
		Name:      chunks[0],
		Namespace: chunks[1],
	}, nil
}
