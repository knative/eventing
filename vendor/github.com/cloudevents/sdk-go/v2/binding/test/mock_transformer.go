package test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudevents/sdk-go/v2/binding"
)

type MockTransformer struct {
	Invoked int
}

func (m *MockTransformer) Transform(binding.MessageMetadataReader, binding.MessageMetadataWriter) error {
	m.Invoked++
	return nil
}

var _ binding.Transformer = (*MockTransformer)(nil)

func AssertTransformerInvokedOneTime(t *testing.T, m *MockTransformer) {
	require.Equal(t,
		1,
		m.Invoked,
		"Transformer must be invoked one time, while it was invoked %d",
		m.Invoked,
	)
}
