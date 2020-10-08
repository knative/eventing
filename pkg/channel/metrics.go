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

package channel

import "go.opencensus.io/tag"

const (
	// LabelUniqueName is the label for the unique name per stats_reporter instance.
	LabelUniqueName = "unique_name"

	// LabelContainerName is the label for the immutable name of the container.
	LabelContainerName = "container_name"
)

var (
	ContainerTagKey = tag.MustNewKey(LabelContainerName)
	UniqueTagKey    = tag.MustNewKey(LabelUniqueName)
)
