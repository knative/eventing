package http

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/format"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/types"
)

// Fill the provided httpRequest with the message m.
// Using context you can tweak the encoding processing (more details on binding.Write documentation).
func WriteRequest(ctx context.Context, m binding.Message, httpRequest *http.Request, transformers ...binding.TransformerFactory) error {
	structuredWriter := (*httpRequestWriter)(httpRequest)
	binaryWriter := (*httpRequestWriter)(httpRequest)

	_, err := binding.Write(
		ctx,
		m,
		structuredWriter,
		binaryWriter,
		transformers...,
	)
	return err
}

type httpRequestWriter http.Request

func (b *httpRequestWriter) SetStructuredEvent(ctx context.Context, format format.Format, event io.Reader) error {
	b.Header.Set(ContentType, format.MediaType())
	b.Body = ioutil.NopCloser(event)
	return nil
}

func (b *httpRequestWriter) Start(ctx context.Context) error {
	return nil
}

func (b *httpRequestWriter) End(ctx context.Context) error {
	return nil
}

func (b *httpRequestWriter) SetData(reader io.Reader) error {
	b.Body = ioutil.NopCloser(reader)
	return nil
}

func (b *httpRequestWriter) SetAttribute(attribute spec.Attribute, value interface{}) error {
	// Http headers, everything is a string!
	s, err := types.Format(value)
	if err != nil {
		return err
	}

	if attribute.Kind() == spec.DataContentType {
		b.Header.Add(ContentType, s)
	} else {
		b.Header.Add(prefix+attribute.Name(), s)
	}
	return nil
}

func (b *httpRequestWriter) SetExtension(name string, value interface{}) error {
	// Http headers, everything is a string!
	s, err := types.Format(value)
	if err != nil {
		return err
	}
	b.Header.Add(prefix+name, s)
	return nil
}

var _ binding.StructuredWriter = (*httpRequestWriter)(nil) // Test it conforms to the interface
var _ binding.BinaryWriter = (*httpRequestWriter)(nil)     // Test it conforms to the interface
