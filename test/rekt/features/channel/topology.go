/*
Copyright 2021 The Knative Authors

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

package channel

import (
	"fmt"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/subscription"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/manifest"
	"strconv"

	"knative.dev/reconciler-test/pkg/feature"
)

type subCfg struct {
	prefix   string
	hasSub   bool
	hasReply bool
	hasDLQ   bool
	delivery *v1.DeliverySpec
}

func (s *subCfg) cuteName(suffix string) string {
	return fmt.Sprintf("%s%s", s.prefix, suffix)
}

//  source --> [channel (chDS)] --+--[sub1 (sub1DS)]--> sub1sub (optional) --> sub1reply (optional)
//                        |       |            |
//                        |       |            +-->sub1dlq (optional)
//                        |      ...
//                        |       +-[sub{n} (sub{n}DS)]--> sub{n}sub (optional)--> sub{n}reply (optional)
//                        |                   |
//                        |                   +-->sub{n}dlq (optional)
//                        |
//                        +--[DLQ]--> chdlq (optional)
//
func createChannelTopology(f *feature.Feature, chName string, chDS *v1.DeliverySpec, subs []subCfg) *eventshub.EventProber {
	prober := eventshub.NewProber()
	// Install the receivers.
	f.Setup("install channel DLQ", prober.ReceiverInstall("chdlq"))

	var chOpts []manifest.CfgFn
	if chDS != nil {
		if chDS.DeadLetterSink != nil {
			chOpts = append(chOpts, delivery.WithDeadLetterSink(prober.AsKReference("chdlq"), ""))
		}
		if chDS.Retry != nil {
			chOpts = append(chOpts, delivery.WithRetry(*chDS.Retry, chDS.BackoffPolicy, chDS.BackoffDelay))
		}
	}
	f.Setup("Create Channel Impl", channel_impl.Install(chName, chOpts...))
	f.Setup("Channel is Ready", channel_impl.IsReady(chName)) // We want to block in the setup phase until the channel is ready to go.

	// Install subscriptions.
	for i, sub := range subs {
		var opts []manifest.CfgFn
		if sub.hasSub {
			f.Setup("install subscription"+strconv.Itoa(i)+" subscriber", prober.ReceiverInstall(sub.cuteName("sub")))
			opts = append(opts, subscription.WithSubscriber(prober.AsKReference(sub.cuteName("sub")), ""))
		}
		if sub.hasReply {
			f.Setup("install subscription"+strconv.Itoa(i)+" reply", prober.ReceiverInstall(sub.cuteName("reply")))
			opts = append(opts, subscription.WithReply(prober.AsKReference(sub.cuteName("reply")), ""))
		}
		if sub.delivery != nil {
			if sub.hasDLQ {
				f.Setup("install subscription"+strconv.Itoa(i)+" DLQ", prober.ReceiverInstall(sub.cuteName("dlq")))
				opts = append(opts, subscription.WithDeadLetterSink(prober.AsKReference(sub.cuteName("dlq")), ""))
			}
			if sub.delivery.Retry != nil {
				opts = append(opts, subscription.WithRetry(*sub.delivery.Retry, sub.delivery.BackoffPolicy, sub.delivery.BackoffDelay))
			}
		}
		f.Setup("install subscription"+strconv.Itoa(i), subscription.Install(sub.prefix, opts...))
	}

	// Create Channel with delivery spec.

	// f.Setup("install subscription2 DLQ", prober.ReceiverInstall("sub2dlq"))

	return prober
}
