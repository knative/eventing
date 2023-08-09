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
	"context"
	"math/rand"
	"net/url"
	"testing"

	"github.com/cloudevents/sdk-go/v2/binding"
	cebindingtest "github.com/cloudevents/sdk-go/v2/binding/test"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/assert"
)

// Test the KnativeErrorTransformers() functionality
func TestKnativeErrorTransformers(t *testing.T) {

	// Test Data
	destinationURL, _ := url.Parse("http://foo.bar.svc.cluster.local")

	// Define the test cases
	testCases := []struct {
		name string
		dest url.URL
		code int
		data string
	}{
		{
			name: "Destination Empty",
			dest: url.URL{},
			code: 500,
			data: "",
		},
		{
			name: "Data Empty",
			dest: *destinationURL,
			code: 500,
			data: "",
		},
		{
			name: "Data Less Than Max Length",
			dest: *destinationURL,
			code: 500,
			data: randomString(t, KnativeErrorDataExtensionMaxLength-1),
		},
		{
			name: "Data Exactly Max Length",
			dest: *destinationURL,
			code: 500,
			data: randomString(t, KnativeErrorDataExtensionMaxLength),
		},
		{
			name: "Data More Than Max Length",
			dest: *destinationURL,
			code: 500,
			data: randomString(t, KnativeErrorDataExtensionMaxLength+1),
		},
	}

	// Loop over the test cases
	for _, testCase := range testCases {

		// Perform an individual test case
		t.Run(testCase.name, func(t *testing.T) {

			// Get the KnativeErrorTransformers for the current testCase
			knativeErrorDataTransformer := KnativeErrorTransformers(testCase.dest, testCase.code, testCase.data)

			// Create the Transformer Input/Want Events
			inputEvent := cetest.MinEvent()
			inputMessage := binding.ToMessage(&inputEvent)
			wantEvent := inputEvent.Clone()
			wantEvent.SetExtension(KnativeErrorDestExtensionKey, testCase.dest)
			wantEvent.SetExtension(KnativeErrorCodeExtensionKey, testCase.code)
			data := testCase.data
			if len(data) > KnativeErrorDataExtensionMaxLength {
				data = data[:KnativeErrorDataExtensionMaxLength]
			}
			wantEvent.SetExtension(KnativeErrorDataExtensionKey, data)

			// Define Transformer tests using the KnativeErrorTransformer
			transformerTests := []cebindingtest.TransformerTestArgs{
				{
					Name:         "Add Extensions To Event",
					InputEvent:   inputEvent,
					WantEvent:    wantEvent,
					Transformers: binding.Transformers{knativeErrorDataTransformer},
				},
				{
					Name:         "Add Extensions To Message",
					InputMessage: inputMessage,
					WantEvent:    wantEvent,
					Transformers: binding.Transformers{knativeErrorDataTransformer},
				},
			}

			// Run the Transformer tests
			cebindingtest.RunTransformerTests(t, context.Background(), transformerTests)
		})
	}
}

// randomString returns a randomly generated string of the specified length
func randomString(t *testing.T, length int) string {
	bytes := make([]byte, length)
	readLen, err := rand.Read(bytes)
	assert.Nil(t, err)
	assert.Equal(t, length, readLen)
	assert.Len(t, bytes, length)
	return string(bytes)
}
