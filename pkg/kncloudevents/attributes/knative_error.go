/*
Copyright 2021 The Knative Authors

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

package attributes

import (
	"net/url"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
)

const (
	KnativeErrorDestExtensionKey       = "knativeerrordest"
	KnativeErrorCodeExtensionKey       = "knativeerrorcode"
	KnativeErrorDataExtensionKey       = "knativeerrordata"
	KnativeErrorDataExtensionMaxLength = 1024
)

// KnativeErrorTransformers returns Transformers which add the specified destination and error code/data extensions.
func KnativeErrorTransformers(destination url.URL, code int, data string) binding.Transformers {
	destTransformer := transformer.AddExtension(KnativeErrorDestExtensionKey, destination)
	codeTransformer := transformer.AddExtension(KnativeErrorCodeExtensionKey, code)
	if len(data) > KnativeErrorDataExtensionMaxLength {
		data = data[:KnativeErrorDataExtensionMaxLength] // Truncate data to max length
	}
	dataTransformer := transformer.AddExtension(KnativeErrorDataExtensionKey, data)
	return binding.Transformers{destTransformer, codeTransformer, dataTransformer}
}
