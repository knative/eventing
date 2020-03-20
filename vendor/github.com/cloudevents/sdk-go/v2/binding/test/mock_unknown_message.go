package test

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/binding"
)

type unknownMessage struct{}

func (u unknownMessage) ReadEncoding() binding.Encoding {
	return binding.EncodingUnknown
}

func (u unknownMessage) ReadStructured(context.Context, binding.StructuredWriter) error {
	return binding.ErrNotStructured
}

func (u unknownMessage) ReadBinary(context.Context, binding.BinaryWriter) error {
	return binding.ErrNotBinary
}

func (u unknownMessage) Finish(error) error {
	return nil
}

var UnknownMessage binding.Message = unknownMessage{}
