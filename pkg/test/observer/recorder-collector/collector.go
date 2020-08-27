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

package recorder_collector

import (
	"context"
	"encoding/json"

	recorder_vent "knative.dev/eventing/pkg/test/observer/recorder-vent"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"knative.dev/eventing/pkg/test/observer"
)

// Filter observed events, return true if the observation should be included.
type FilterFn func(ob observer.Observed) bool

type Collector interface {
	// List all observed events from a ref, optionally filter (pass filter, && together)
	List(from duckv1.KReference, filters ...FilterFn) ([]observer.Observed, error)
}

func New(ctx context.Context) Collector {
	return &collector{client: kubeclient.Get(ctx)}
}

type collector struct {
	client kubernetes.Interface
}

func (c *collector) List(from duckv1.KReference, filters ...FilterFn) ([]observer.Observed, error) {
	var lister v1.EventInterface
	if from.Kind == "Namespace" {
		lister = c.client.CoreV1().Events(from.Name)
	} else {
		lister = c.client.CoreV1().Events(from.Namespace) // TODO: I do not understand how to do cluster scoped objects.
	}
	events, err := lister.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	obs := make([]observer.Observed, 0)

	for _, v := range events.Items {
		switch v.Reason {
		case recorder_vent.EventReason:
			ob := observer.Observed{}
			if err := json.Unmarshal([]byte(v.Message), &ob); err != nil {
				return nil, err
			}
			if filters != nil {
				skip := false
				for _, fn := range filters {
					if !fn(ob) {
						skip = true
					}
				}
				if skip {
					continue
				}
			}

			obs = append(obs, ob)
			// TODO: worry about v.Count
		}
	}

	// sort by time collected
	return obs, nil
}
