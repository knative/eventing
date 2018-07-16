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

package sample

import (
	"context"
	"log"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconcile ReplicaSets
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the ReplicaSet from the cache
	rs := &appsv1.ReplicaSet{}
	err := r.client.Get(context.TODO(), request.NamespacedName, rs)
	if errors.IsNotFound(err) {
		log.Printf("could not find ReplicaSet %v.\n", request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		log.Printf("could not fetch ReplicaSet %v for %+v\n", err, request)
		return reconcile.Result{}, err
	}

	// Print the ReplicaSet
	log.Printf("replicaSet Name %s Namespace %s, Pod Name: %s\n",
		rs.Name, rs.Namespace, rs.Spec.Template.Spec.Containers[0].Name)

	// Set the label if it is missing
	if rs.Labels == nil {
		rs.Labels = map[string]string{}
	}
	if rs.Labels["hello"] == "world" {
		return reconcile.Result{}, nil
	}

	// Update the ReplicaSet
	rs.Labels["hello"] = "world"
	err = r.client.Update(context.TODO(), rs)
	if err != nil {
		log.Printf("could not write ReplicaSet %v\n", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
