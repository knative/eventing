/*
Copyright 2023 The CloudEvents Authors
SPDX-License-Identifier: Apache-2.0
*/

package mqtt_paho

import (
	"bytes"
	"context"
	"io"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/format"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/types"
	"github.com/eclipse/paho.golang/paho"
)

// WritePubMessage fills the provided pubMessage with the message m.
// Using context you can tweak the encoding processing (more details on binding.Write documentation).
func WritePubMessage(ctx context.Context, m binding.Message, pubMessage *paho.Publish, transformers ...binding.Transformer) error {
	structuredWriter := (*pubMessageWriter)(pubMessage)
	binaryWriter := (*pubMessageWriter)(pubMessage)

	_, err := binding.Write(
		ctx,
		m,
		structuredWriter,
		binaryWriter,
		transformers...,
	)
	return err
}

type pubMessageWriter paho.Publish

var (
	_ binding.StructuredWriter = (*pubMessageWriter)(nil)
	_ binding.BinaryWriter     = (*pubMessageWriter)(nil)
)

func (b *pubMessageWriter) SetStructuredEvent(ctx context.Context, f format.Format, event io.Reader) error {
	if b.Properties == nil {
		b.Properties = &paho.PublishProperties{
			User: make([]paho.UserProperty, 0),
		}
	}
	b.Properties.User.Add(contentType, f.MediaType())
	var buf bytes.Buffer
	_, err := io.Copy(&buf, event)
	if err != nil {
		return err
	}
	b.Payload = buf.Bytes()
	return nil
}

func (b *pubMessageWriter) Start(ctx context.Context) error {
	if b.Properties == nil {
		b.Properties = &paho.PublishProperties{}
	}
	// the UserProperties of publish message is used to load event extensions
	b.Properties.User = make([]paho.UserProperty, 0)
	return nil
}

func (b *pubMessageWriter) End(ctx context.Context) error {
	return nil
}

func (b *pubMessageWriter) SetData(reader io.Reader) error {
	buf, ok := reader.(*bytes.Buffer)
	if !ok {
		buf = new(bytes.Buffer)
		_, err := io.Copy(buf, reader)
		if err != nil {
			return err
		}
	}
	b.Payload = buf.Bytes()
	return nil
}

func (b *pubMessageWriter) SetAttribute(attribute spec.Attribute, value interface{}) error {
	if attribute.Kind() == spec.DataContentType {
		if value == nil {
			b.removeProperty(contentType)
		}
		s, err := types.Format(value)
		if err != nil {
			return err
		}
		if err := b.addProperty(contentType, s); err != nil {
			return err
		}
	} else {
		if value == nil {
			b.removeProperty(prefix + attribute.Name())
		}
		return b.addProperty(prefix+attribute.Name(), value)
	}
	return nil
}

func (b *pubMessageWriter) SetExtension(name string, value interface{}) error {
	if value == nil {
		b.removeProperty(prefix + name)
	}
	return b.addProperty(prefix+name, value)
}

func (b *pubMessageWriter) removeProperty(key string) {
	for i, v := range b.Properties.User {
		if v.Key == key {
			b.Properties.User = append(b.Properties.User[:i], b.Properties.User[i+1:]...)
			break
		}
	}
}

func (b *pubMessageWriter) addProperty(key string, value interface{}) error {
	s, err := types.Format(value)
	if err != nil {
		return err
	}

	b.Properties.User = append(b.Properties.User, paho.UserProperty{
		Key:   key,
		Value: s,
	})
	return nil
}
