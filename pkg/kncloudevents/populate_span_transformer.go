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

package kncloudevents

import (
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/types"
	"go.opencensus.io/trace"
)

func PopulateSpan(span *trace.Span) binding.TransformerFunc {
	return func(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
		_, specVersion := reader.GetAttribute(spec.SpecVersion)
		if specVersion != nil {
			specVersionParsed, err := types.Format(specVersion)
			if err != nil {
				return err
			}
			span.AddAttributes(trace.StringAttribute("cloudevents.specversion", specVersionParsed))
		}

		_, id := reader.GetAttribute(spec.ID)
		if id != nil {
			idParsed, err := types.Format(id)
			if err != nil {
				return err
			}
			span.AddAttributes(trace.StringAttribute("cloudevents.id", idParsed))
		}

		_, ty := reader.GetAttribute(spec.Type)
		if ty != nil {
			tyParsed, err := types.Format(ty)
			if err != nil {
				return err
			}
			span.AddAttributes(trace.StringAttribute("cloudevents.type", tyParsed))
		}

		_, source := reader.GetAttribute(spec.Source)
		if source != nil {
			sourceParsed, err := types.Format(source)
			if err != nil {
				return err
			}
			span.AddAttributes(trace.StringAttribute("cloudevents.source", sourceParsed))
		}

		return nil
	}
}
