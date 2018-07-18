/*
Copyright 2018 The Knative Authors

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

package feed

import (
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const controllerAgentName = "feed-controller"

type reconciler struct {
	client   client.Client
	recorder record.EventRecorder
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a Feed controller.
func ProvideController(mrg manager.Manager) (controller.Controller, error) {
	// Setup a new controller to Reconcile Feeds.
	c, err := controller.New(controllerAgentName, mrg, controller.Options{
		Reconciler: &reconciler{
			recorder: mrg.GetRecorder(controllerAgentName),
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch Feed events and enqueue Feed object key.
	if err := c.Watch(&source.Kind{Type: &feedsv1alpha1.Feed{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	// Watch Jobs and enqueue owning Feed key.
	if err := c.Watch(&source.Kind{Type: &batchv1.Job{}},
		&handler.EnqueueRequestForOwner{OwnerType: &feedsv1alpha1.Feed{}, IsController: true}); err != nil {
		return nil, err
	}

	return c, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
