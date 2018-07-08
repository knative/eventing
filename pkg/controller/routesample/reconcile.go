/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package routesample

import (
	"context"
	"log"

	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconcileRoute struct {
	client client.Client
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconcileRoute{}

// Reconcile Routes
func (r *reconcileRoute) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Route from the cache
	route := &servingv1alpha1.Route{}
	err := r.client.Get(context.TODO(), request.NamespacedName, route)
	if errors.IsNotFound(err) {
		log.Printf("Could not find Route %v.\n", request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		log.Printf("Could not fetch Route %v for %+v\n", err, request)
		return reconcile.Result{}, err
	}

	// Print the Route
	log.Printf("Route Name %s Namespace %s\n",
		route.Name, route.Namespace)

	// Set the label if it is missing
	if route.Labels == nil {
		route.Labels = map[string]string{}
	}
	if route.Labels["hello"] == "world" {
		return reconcile.Result{}, nil
	}

	// Update the Route
	route.Labels["hello"] = "world"
	err = r.client.Update(context.TODO(), route)
	if err != nil {
		log.Printf("Could not write Route %v\n", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
