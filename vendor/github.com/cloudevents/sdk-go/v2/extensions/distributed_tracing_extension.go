/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package extensions

import (
	"reflect"
	"strings"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"

	"github.com/cloudevents/sdk-go/v2/types"
)

const (
	TraceParentExtension = "traceparent"
	TraceStateExtension  = "tracestate"
)

// DistributedTracingExtension represents the extension for cloudevents context
type DistributedTracingExtension struct {
	TraceParent string `json:"traceparent"`
	TraceState  string `json:"tracestate"`
}

// AddTracingAttributes adds the tracing attributes traceparent and tracestate to the cloudevents context
func (d DistributedTracingExtension) AddTracingAttributes(e event.EventWriter) {
	if d.TraceParent != "" {
		value := reflect.ValueOf(d)
		typeOf := value.Type()

		for i := 0; i < value.NumField(); i++ {
			k := strings.ToLower(typeOf.Field(i).Name)
			v := value.Field(i).Interface()
			if k == TraceStateExtension && v == "" {
				continue
			}
			e.SetExtension(k, v)
		}
	}
}

func GetDistributedTracingExtension(event event.Event) (DistributedTracingExtension, bool) {
	if tp, ok := event.Extensions()[TraceParentExtension]; ok {
		if tpStr, err := types.ToString(tp); err == nil {
			var tsStr string
			if ts, ok := event.Extensions()[TraceStateExtension]; ok {
				tsStr, _ = types.ToString(ts)
			}
			return DistributedTracingExtension{TraceParent: tpStr, TraceState: tsStr}, true
		}
	}
	return DistributedTracingExtension{}, false
}

func (d *DistributedTracingExtension) ReadTransformer() binding.TransformerFunc {
	return func(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
		tp := reader.GetExtension(TraceParentExtension)
		if tp != nil {
			tpFormatted, err := types.Format(tp)
			if err != nil {
				return err
			}
			d.TraceParent = tpFormatted
		}
		ts := reader.GetExtension(TraceStateExtension)
		if ts != nil {
			tsFormatted, err := types.Format(ts)
			if err != nil {
				return err
			}
			d.TraceState = tsFormatted
		}
		return nil
	}
}

func (d *DistributedTracingExtension) WriteTransformer() binding.TransformerFunc {
	return func(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
		err := writer.SetExtension(TraceParentExtension, d.TraceParent)
		if err != nil {
			return nil
		}
		if d.TraceState != "" {
			return writer.SetExtension(TraceStateExtension, d.TraceState)
		}
		return nil
	}
}
