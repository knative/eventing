package test

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/format"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/event"
)

type MockTransformerFactory struct {
	FailStructured bool
	FailBinary     bool

	StructuredDelegate           binding.StructuredWriter
	InvokedStructuredFactory     int
	InvokedStructuredTransformer int

	BinaryDelegate           binding.BinaryWriter
	InvokedBinaryFactory     int
	InvokedBinaryTransformer int

	InvokedEventFactory     int
	InvokedEventTransformer int
}

func NewMockTransformerFactory(failStructured bool, failBinary bool) *MockTransformerFactory {
	return &MockTransformerFactory{
		FailStructured: failStructured,
		FailBinary:     failBinary,
	}
}

func (m *MockTransformerFactory) StructuredTransformer(encoder binding.StructuredWriter) binding.StructuredWriter {
	m.InvokedStructuredFactory++
	if m.FailStructured {
		return nil
	}
	m.StructuredDelegate = encoder
	return m
}

func (m *MockTransformerFactory) BinaryTransformer(encoder binding.BinaryWriter) binding.BinaryWriter {
	m.InvokedBinaryFactory++
	if m.FailBinary {
		return nil
	}
	m.BinaryDelegate = encoder
	return m
}

func (m *MockTransformerFactory) EventTransformer() binding.EventTransformer {
	m.InvokedEventFactory++
	return func(event *event.Event) error {
		m.InvokedEventTransformer++
		return nil
	}
}

func (m *MockTransformerFactory) SetStructuredEvent(ctx context.Context, format format.Format, event io.Reader) error {
	m.InvokedStructuredTransformer++
	return m.StructuredDelegate.SetStructuredEvent(ctx, format, event)
}

func (m *MockTransformerFactory) Start(ctx context.Context) error {
	m.InvokedBinaryTransformer++
	return m.BinaryDelegate.Start(ctx)
}

func (m *MockTransformerFactory) SetAttribute(attribute spec.Attribute, value interface{}) error {
	return m.BinaryDelegate.SetAttribute(attribute, value)
}

func (m *MockTransformerFactory) SetExtension(name string, value interface{}) error {
	return m.BinaryDelegate.SetExtension(name, value)
}

func (m *MockTransformerFactory) SetData(data io.Reader) error {
	return m.BinaryDelegate.SetData(data)
}

func (m *MockTransformerFactory) End(ctx context.Context) error {
	return m.BinaryDelegate.End(ctx)
}

func AssertTransformerInvokedOneTime(t *testing.T, m *MockTransformerFactory) {
	require.Equal(t,
		1,
		m.InvokedStructuredTransformer+m.InvokedBinaryTransformer+m.InvokedEventTransformer,
		"Transformer must be invoked one time, while it was invoked structured %d, binary %d, event %d",
		m.InvokedStructuredTransformer,
		m.InvokedBinaryTransformer,
		m.InvokedEventTransformer,
	)
}
