package transformer

import (
	"time"

	"github.com/google/uuid"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/types"
)

var (
	// Sets the cloudevents id attribute to a UUID.New()
	SetUUID binding.Transformer = setUUID{}
	// Add the cloudevents time attribute, if missing, to time.Now()
	AddTimeNow binding.Transformer = addTimeNow{}
)

type setUUID struct{}

func (a setUUID) Transform(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
	attr, _ := reader.GetAttribute(spec.ID)
	return writer.SetAttribute(attr, uuid.New().String())
}

type addTimeNow struct{}

func (a addTimeNow) Transform(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
	attr, ti := reader.GetAttribute(spec.Time)
	if ti == nil {
		return writer.SetAttribute(attr, types.Timestamp{Time: time.Now()})
	}
	return nil
}
