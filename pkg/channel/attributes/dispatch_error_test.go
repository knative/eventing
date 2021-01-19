package attributes

import (
	"context"
	"encoding/json"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
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

// Test The SetDispatchErrorExtension() Functionality
func TestSetDispatchErrorExtension(t *testing.T) {

	// Test Data
	sampleExtensionKey := "sampleextensionkey"
	ctx := context.TODO()
	code := 99
	data := []byte("TestData")
	dispatchErrorExtension := NewDispatchErrorExtension(code, data)

	// Create An Event / Message
	origEvent := cloudevents.NewEvent()
	origEvent.SetSource("example/uri")
	origEvent.SetType("example.type")
	origEvent.SetExtension(sampleExtensionKey, "SampleExtensionValue")
	_ = origEvent.SetData(cloudevents.ApplicationJSON, map[string]string{"hello": "world"})
	origMessage := binding.ToMessage(&origEvent)

	// Perform The Test
	resultMessage, err := SetDispatchErrorExtension(ctx, dispatchErrorExtension, origMessage)

	// Validate The Results
	assert.Nil(t, err)
	assert.NotNil(t, resultMessage)
	resultEvent, err := binding.ToEvent(ctx, resultMessage)
	assert.Nil(t, err)
	assert.NotNil(t, resultEvent)
	assert.Equal(t, origEvent.Source(), resultEvent.Source())
	assert.Equal(t, origEvent.Type(), resultEvent.Type())
	assert.Equal(t, origEvent.Data(), resultEvent.Data())
	assert.Equal(t, origEvent.Extensions()[sampleExtensionKey], resultEvent.Extensions()[sampleExtensionKey])
	resultDispatchErrorExtensionBytes, ok := resultEvent.Extensions()[DispatchErrorExtensionKey].([]byte)
	assert.True(t, ok)
	resultDispatchErrorExtension := &DispatchErrorExtension{}
	err = json.Unmarshal(resultDispatchErrorExtensionBytes, resultDispatchErrorExtension)
	assert.Nil(t, err)
	assert.Equal(t, dispatchErrorExtension.Code, resultDispatchErrorExtension.Code)
	assert.Equal(t, dispatchErrorExtension.Data, resultDispatchErrorExtension.Data)
}
