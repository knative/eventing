/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package buffering

import (
	"bytes"
	"context"
	"io"

	"github.com/valyala/bytebufferpool"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/format"
)

var structMessagePool bytebufferpool.Pool

// structBufferedMessage implements a structured-mode message as a simple struct.
// This message implementation is used by CopyMessage and BufferMessage
type structBufferedMessage struct {
	Format format.Format
	Bytes  *bytebufferpool.ByteBuffer
}

func (m *structBufferedMessage) ReadEncoding() binding.Encoding {
	return binding.EncodingStructured
}

// Structured copies structured data to a StructuredWriter
func (m *structBufferedMessage) ReadStructured(ctx context.Context, enc binding.StructuredWriter) error {
	return enc.SetStructuredEvent(ctx, m.Format, bytes.NewReader(m.Bytes.B))
}

// Binary returns ErrNotBinary
func (m structBufferedMessage) ReadBinary(context.Context, binding.BinaryWriter) error {
	return binding.ErrNotBinary
}

func (m *structBufferedMessage) Finish(error) error {
	structMessagePool.Put(m.Bytes)
	return nil
}

func (m *structBufferedMessage) SetStructuredEvent(ctx context.Context, format format.Format, event io.Reader) error {
	m.Bytes = structMessagePool.Get()
	_, err := io.Copy(m.Bytes, event)
	if err != nil {
		return err
	}
	m.Format = format
	return nil
}

var _ binding.Message = (*structBufferedMessage)(nil)          // Test it conforms to the interface
var _ binding.StructuredWriter = (*structBufferedMessage)(nil) // Test it conforms to the interface
