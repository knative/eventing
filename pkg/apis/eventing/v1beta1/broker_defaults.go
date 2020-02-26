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

package v1beta1

import (
	"context"
	"fmt"

	"knative.dev/eventing/pkg/apis/config"
	"knative.dev/pkg/apis"
)

func (b *Broker) SetDefaults(ctx context.Context) {
	// TODO(vaikas): Set the default class annotation if not specified
	withNS := apis.WithinParent(ctx, b.ObjectMeta)
	b.Spec.SetDefaults(withNS)
}

func (bs *BrokerSpec) SetDefaults(ctx context.Context) {
	if bs.Config != nil {
		return
	}

	cfg := config.FromContextOrDefaults(ctx)
	fmt.Printf("GOT CONTEXT AS: %+v", cfg)
	c, err := cfg.Defaults.GetBrokerConfig(apis.ParentMeta(ctx).Namespace)
	if err != nil {
		bs.Config = c
	}
}
