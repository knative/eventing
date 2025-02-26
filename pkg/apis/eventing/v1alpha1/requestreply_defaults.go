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

	"k8s.io/utils/ptr"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/pkg/apis"
)

func (rr *RequestReply) SetDefaults(ctx context.Context) {
	ctx = apis.WithinParent(ctx, rr.ObjectMeta)
	rr.Spec.SetDefaults(ctx)
}

func (rrs *RequestReplySpec) SetDefaults(ctx context.Context) {
	if rrs.Timeout == nil || *rrs.Timeout == "" {
		rrs.Timeout = ptr.To(feature.FromContextOrDefaults(ctx).RequestReplyDefaultTimeout())
	}

	if rrs.CorrelationAttribute == "" {
		rrs.CorrelationAttribute = "correlationid"
	}

	if rrs.ReplyAttribute == "" {
		rrs.ReplyAttribute = "replyid"
	}
}
