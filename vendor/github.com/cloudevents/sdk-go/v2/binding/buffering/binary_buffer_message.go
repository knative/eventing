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
	"github.com/cloudevents/sdk-go/v2/binding/spec"
)

var binaryMessagePool bytebufferpool.Pool

// binaryBufferedMessage implements a binary-mode message as a simple struct.
// This message implementation is used by CopyMessage and BufferMessage
type binaryBufferedMessage struct {
	version    spec.Version
	metadata   map[spec.Attribute]interface{}
	extensions map[string]interface{}
	body       *bytebufferpool.ByteBuffer
}

func (m *binaryBufferedMessage) Start(ctx context.Context) error {
	m.metadata = make(map[spec.Attribute]interface{}, 4)
	m.extensions = make(map[string]interface{})
	return nil
}

func (m *binaryBufferedMessage) ReadEncoding() binding.Encoding {
	return binding.EncodingBinary
}

func (m *binaryBufferedMessage) ReadStructured(context.Context, binding.StructuredWriter) error {
	return binding.ErrNotStructured
}

func (m *binaryBufferedMessage) ReadBinary(ctx context.Context, b binding.BinaryWriter) (err error) {
	for k, v := range m.metadata {
		err = b.SetAttribute(k, v)
		if err != nil {
			return
		}
	}
	for k, v := range m.extensions {
		err = b.SetExtension(k, v)
		if err != nil {
			return
		}
	}
	if m.body != nil {
		err = b.SetData(bytes.NewReader(m.body.Bytes()))
		if err != nil {
			return
		}
	}
	return nil
}

func (m *binaryBufferedMessage) Finish(error) error {
	if m.body != nil {
		binaryMessagePool.Put(m.body)
	}
	return nil
}

// Binary Encoder
func (m *binaryBufferedMessage) SetData(data io.Reader) error {
	buf := binaryMessagePool.Get()
	w, err := io.Copy(buf, data)
	if err != nil {
		return err
	}
	if w == 0 {
		binaryMessagePool.Put(buf)
		return nil
	}
	m.body = buf
	return nil
}

func (m *binaryBufferedMessage) SetAttribute(attribute spec.Attribute, value interface{}) error {
	// If spec version we need to change to right context struct
	m.version = attribute.Version()
	m.metadata[attribute] = value
	return nil
}

func (m *binaryBufferedMessage) SetExtension(name string, value interface{}) error {
	m.extensions[name] = value
	return nil
}

func (m *binaryBufferedMessage) End(ctx context.Context) error {
	return nil
}

func (m *binaryBufferedMessage) GetAttribute(k spec.Kind) (spec.Attribute, interface{}) {
	a := m.version.AttributeFromKind(k)
	if a != nil {
		return a, m.metadata[a]
	}
	return nil, nil
}

func (m *binaryBufferedMessage) GetExtension(name string) interface{} {
	return m.extensions[name]
}

var _ binding.Message = (*binaryBufferedMessage)(nil) // Test it conforms to the interface
var _ binding.MessageMetadataReader = (*binaryBufferedMessage)(nil)
var _ binding.BinaryWriter = (*binaryBufferedMessage)(nil)
