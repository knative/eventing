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
	"github.com/cloudevents/sdk-go/v2/binding"
	"go.uber.org/zap"
)

const (
	KnativeErrorCodeExtensionKey       = "knativeerrorcode"
	KnativeErrorDataExtensionKey       = "knativeerrordata"
	KnativeErrorDataExtensionMaxLength = 1024
)

// KnativeErrorCodeTransformer returns a CloudEvent TransformerFunc which sets the specified error code extension.
func KnativeErrorCodeTransformer(logger *zap.Logger, code int, ignoreErrors bool) binding.TransformerFunc {
	return func(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
		err := writer.SetExtension(KnativeErrorCodeExtensionKey, code)
		if err != nil {
			if logger != nil {
				logger.Error("Failed to set knative error code extension",
					zap.String("ExtensionKey", KnativeErrorCodeExtensionKey),
					zap.Int("ExtensionValue", code),
					zap.Bool("IgnoreErrors", ignoreErrors),
					zap.Error(err))
			}
			if !ignoreErrors {
				return err
			}
		}
		return nil
	}
}

// KnativeErrorDataTransformer returns a CloudEvent TransformerFunc which sets the specified error data extension.
func KnativeErrorDataTransformer(logger *zap.Logger, data string, ignoreErrors bool) binding.TransformerFunc {
	return func(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
		dataLength := len(data)
		if dataLength > 0 {
			if dataLength > KnativeErrorDataExtensionMaxLength {
				data = data[:KnativeErrorDataExtensionMaxLength] // Truncate data to max length
			}
			err := writer.SetExtension(KnativeErrorDataExtensionKey, data)
			if err != nil {
				if logger != nil {
					logger.Error("Failed to set knative error extension",
						zap.String("ExtensionKey", KnativeErrorDataExtensionKey),
						zap.String("ExtensionValue", data),
						zap.Bool("IgnoreErrors", ignoreErrors),
						zap.Error(err))
				}
				if !ignoreErrors {
					return err
				}
			}
		}
		return nil
	}
}
