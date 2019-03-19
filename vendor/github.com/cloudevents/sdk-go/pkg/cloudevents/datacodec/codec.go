package datacodec

import (
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec/json"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec/xml"
)

type Decoder func(in, out interface{}) error
type Encoder func(in interface{}) ([]byte, error)

var decoder map[string]Decoder
var encoder map[string]Encoder

func init() {
	decoder = make(map[string]Decoder, 10)
	encoder = make(map[string]Encoder, 10)

	AddDecoder("", json.Decode)
	AddDecoder("application/json", json.Decode)
	AddDecoder("application/xml", xml.Decode)

	AddEncoder("", json.Encode)
	AddEncoder("application/json", json.Encode)
	AddEncoder("application/xml", xml.Encode)
}

func AddDecoder(contentType string, fn Decoder) {
	decoder[contentType] = fn
}

func AddEncoder(contentType string, fn Encoder) {
	encoder[contentType] = fn
}

func Decode(contentType string, in, out interface{}) error {
	if fn, ok := decoder[contentType]; ok {
		return fn(in, out)
	}
	return fmt.Errorf("[decode] unsupported content type: %q", contentType)
}

func Encode(contentType string, in interface{}) ([]byte, error) {
	if fn, ok := encoder[contentType]; ok {
		return fn(in)
	}
	return nil, fmt.Errorf("[encode] unsupported content type: %q", contentType)
}
