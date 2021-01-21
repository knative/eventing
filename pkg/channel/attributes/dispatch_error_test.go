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

package attributes

import (
	"context"
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/cloudevents/sdk-go/v2/binding"
	cebindingtest "github.com/cloudevents/sdk-go/v2/binding/test"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/assert"
)

// Test The NewDispatchErrorExtension() Functionality
func TestNewDispatchErrorExtension(t *testing.T) {

	// Test Data
	code := 99
	data := []byte("TestData")

	// Perform The Test
	dispatchErrorExtension := NewDispatchErrorExtension(code, data)

	// Validate Results
	assert.NotNil(t, dispatchErrorExtension)
	assert.Equal(t, code, dispatchErrorExtension.Code)
	assert.Equal(t, data, dispatchErrorExtension.Data)
}

// Test TheDispatchErrorExtension's AddExtensionTransformer() Logic
func TestAddExtensionTransformer(t *testing.T) {

	// Test Data
	code := 500

	// Define The TestCase
	type TestCase struct {
		name                   string
		dispatchErrorExtension DispatchErrorExtension
	}

	// Create The TestCases
	testCases := []TestCase{
		{
			name:                   "Nil Data",
			dispatchErrorExtension: NewDispatchErrorExtension(code, nil),
		},
		{
			name:                   "Empty Data",
			dispatchErrorExtension: NewDispatchErrorExtension(code, randomByteArray(t, 0)),
		},
		{
			name:                   "Data Length Less Than Max",
			dispatchErrorExtension: NewDispatchErrorExtension(code, randomByteArray(t, MaxDispatchErrorExtensionDataBytes-1)),
		},
		{
			name:                   "Data Length Exactly Max",
			dispatchErrorExtension: NewDispatchErrorExtension(code, randomByteArray(t, MaxDispatchErrorExtensionDataBytes)),
		},
		{
			name:                   "Data Length Exceeded Max",
			dispatchErrorExtension: NewDispatchErrorExtension(code, randomByteArray(t, MaxDispatchErrorExtensionDataBytes+1)),
		},
	}

	// Execute The Individual Test Cases
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			// Get The DispatchErrorExtension To Test
			testDispatchErrorExtension := testCase.dispatchErrorExtension

			// Copy The DispatchErrorExtension To Ensure Immutability
			origDispatchErrorExtension := NewDispatchErrorExtension(testDispatchErrorExtension.Code, testDispatchErrorExtension.Data)

			// Perform The Test
			testTransformer, err := testDispatchErrorExtension.AddExtensionTransformer()

			// Validate The Results
			assert.Nil(t, err)
			assert.NotNil(t, testTransformer)
			assert.Equal(t, origDispatchErrorExtension, testDispatchErrorExtension)

			// Get The Expected (Possibly Truncated) DispatchErrorExtension
			expectedDispatchErrorExtension := testDispatchErrorExtension
			if len(testDispatchErrorExtension.Data) > MaxDispatchErrorExtensionDataBytes {
				expectedDispatchErrorExtension = NewDispatchErrorExtension(testDispatchErrorExtension.Code, testDispatchErrorExtension.Data[:MaxDispatchErrorExtensionDataBytes])
			}
			expectedDispatchErrorExtensionJsonBytes, err := json.Marshal(expectedDispatchErrorExtension)
			assert.Nil(t, err)

			// Create The Transformer Input/Want Events
			inputEvent := cetest.MinEvent()
			inputMessage := binding.ToMessage(&inputEvent)
			wantEvent := inputEvent.Clone()
			wantEvent.SetExtension(DispatchErrorExtensionKey, expectedDispatchErrorExtensionJsonBytes)

			// Define The Transformer Tests
			transformerTests := []cebindingtest.TransformerTestArgs{
				{
					Name:         "Add Extension To Event",
					InputEvent:   inputEvent,
					WantEvent:    wantEvent,
					Transformers: binding.Transformers{testTransformer},
				},
				{
					Name:         "Add Extension To Message",
					InputMessage: inputMessage,
					WantEvent:    wantEvent,
					Transformers: binding.Transformers{testTransformer},
				},
			}

			// Run The Transformer Tests
			cebindingtest.RunTransformerTests(t, context.Background(), transformerTests)
		})
	}
}

// Utility Function To Generate A Specific Sized Byte Array
func randomByteArray(t *testing.T, length int) []byte {
	bytes := make([]byte, length)
	readLen, err := rand.Read(bytes)
	assert.Nil(t, err)
	assert.Equal(t, length, readLen)
	assert.Len(t, bytes, length)
	return bytes
}
