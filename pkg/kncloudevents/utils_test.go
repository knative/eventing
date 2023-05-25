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

package kncloudevents

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/cloudevents/sdk-go/v2/binding/test"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/stretchr/testify/assert"
)

// Test additional headers are added to the request
func TestWriteRequestWithAdditionalHeadersWritesIntoRequest(t *testing.T) {
	ctx := context.TODO()
	ceSource := "knative.dev/eventing/kncloudevents/receive/test"
	ceType := "knative.dev.kncloudevents.test.sent"
	ceData := "some-foo-string"
	url, err := apis.ParseURL("http://foobar:12345")
	assert.NoError(t, err)

	request, err := NewCloudEventRequest(ctx, duckv1.Addressable{URL: url})
	assert.NoError(t, err)

	ceEvent := cloudevents.NewEvent()
	ceEvent.SetType(ceType)
	ceEvent.SetSource(ceSource)
	_ = ceEvent.SetData(cloudevents.TextPlain, ceData)

	message := binding.ToMessage(&ceEvent)
	defer message.Finish(nil)

	err = WriteRequestWithAdditionalHeaders(ctx, message, request, http.Header{})
	assert.NoError(t, err)

	assert.Equal(t, []string{ceSource}, request.Header["Ce-Source"])
	assert.Equal(t, []string{ceType}, request.Header["Ce-Type"])
	assert.Equal(t, []string{cloudevents.TextPlain}, request.Header["Content-Type"])
	gotPayload, err := io.ReadAll(request.Body)
	assert.NoError(t, err)
	assert.Equal(t, []byte(ceData), gotPayload)

}

// Test additional headers are added to the request
func TestWriteRequestWithAdditionalHeadersAddsHeadersToRequest(t *testing.T) {
	ctx := context.TODO()
	url, err := apis.ParseURL("http://foobar:12345")
	assert.NoError(t, err)

	request, err := NewCloudEventRequest(ctx, duckv1.Addressable{URL: url})
	assert.NoError(t, err)

	ceEvent := cloudevents.NewEvent()
	message := binding.ToMessage(&ceEvent)
	defer message.Finish(nil)

	additionalHeaders := http.Header{}
	additionalHeaders["Some-Key"] = []string{"some-value"}
	additionalHeaders["Another-Key"] = []string{"another-value"}

	err = WriteRequestWithAdditionalHeaders(ctx, message, request, additionalHeaders)
	assert.NoError(t, err)

	assert.Equal(t, additionalHeaders["Some-Key"], request.Header["Some-Key"])
	assert.Equal(t, additionalHeaders["Another-Key"], request.Header["Another-Key"])

}

// If reader's message does have Type attribute
func TestTypeExtractorTransformerWithType(t *testing.T) {
	ceType := "some.custom.type"
	ceEvent := cloudevents.NewEvent()
	ceEvent.SetType(ceType)

	var te = TypeExtractorTransformer("")
	mockBinaryMsg := test.MustCreateMockBinaryMessage(ceEvent)
	te.Transform(mockBinaryMsg.(binding.MessageMetadataReader), mockBinaryMsg.(binding.MessageMetadataWriter))

	assert.Equal(t, ceType, string(te))
}

// If reader's message does NOT have Type attribute
func TestTypeExtractorTransformerWithoutType(t *testing.T) {
	ceEvent := cloudevents.NewEvent()

	var te = TypeExtractorTransformer("")
	mockBinaryMsg := test.MustCreateMockBinaryMessage(ceEvent)
	te.Transform(mockBinaryMsg.(binding.MessageMetadataReader), mockBinaryMsg.(binding.MessageMetadataWriter))

	assert.Equal(t, "", string(te))
}
