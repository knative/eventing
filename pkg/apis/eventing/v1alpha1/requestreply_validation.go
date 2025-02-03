/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"strings"

	"github.com/rickb777/date/period"

	"knative.dev/pkg/apis"
)

func (rr *RequestReply) Validate(ctx context.Context) *apis.FieldError {
	ctx = apis.WithinParent(ctx, rr.ObjectMeta)
	return rr.Spec.Validate(ctx).ViaField("spec")
}

func (rrs *RequestReplySpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if ke := rrs.BrokerRef.Validate(ctx); ke != nil {
		errs = errs.Also(ke.ViaField("brokerRef"))
	}

	if !strings.EqualFold(rrs.BrokerRef.Kind, "broker") {
		errs = errs.Also(apis.ErrInvalidValue(rrs.BrokerRef.Kind, ".kind", "brokerRef kind must be Broker").ViaField("brokerRef"))
	}

	if rrs.BrokerRef.Namespace != "" {
		errs = errs.Also(apis.ErrDisallowedFields("namespace").ViaField("brokerRef"))
	}

	if rrs.Delivery != nil {
		if de := rrs.Delivery.Validate(ctx); de != nil {
			errs = errs.Also(de.ViaField("delivery"))
		}
	}

	if rrs.Timeout != nil {
		timeout, err := period.Parse(*rrs.Timeout)
		if err != nil || timeout.IsZero() || timeout.IsNegative() {
			errs = errs.Also(apis.ErrInvalidValue(*rrs.Timeout, "timeout"))
		}

	}

	if len(rrs.Secrets) == 0 {
		errs = errs.Also(apis.ErrInvalidValue(rrs.Secrets, "secrets", "one or more secrets must be provided"))
	}

	if rrs.CorrelationAttribute == "" ||
		rrs.CorrelationAttribute == "id" ||
		rrs.CorrelationAttribute == "course" ||
		rrs.CorrelationAttribute == "specversion" ||
		rrs.CorrelationAttribute == "type" {
		errs = errs.Also(apis.ErrInvalidValue(rrs.CorrelationAttribute, "correlationattribute", "correlationattribute must be non-empty and cannot be a core cloudevent attribute (id, type, specversion, source)"))
	}

	if rrs.ReplyAttribute == "" ||
		rrs.ReplyAttribute == "id" ||
		rrs.ReplyAttribute == "course" ||
		rrs.ReplyAttribute == "specversion" ||
		rrs.ReplyAttribute == "type" {
		errs = errs.Also(apis.ErrInvalidValue(rrs.ReplyAttribute, "replyattribute", "replyattribute must be non-empty and cannot be a core cloudevent attribute (id, type, specversion, source)"))
	}

	return errs
}
