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

package controller

import (
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/controller/sdk"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "container-provisioner-controller"
)

// ProvideController returns a Subscription controller.
func ProvideController(mgr manager.Manager) (controller.Controller, error) {
	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &v1alpha1.Source{},
		Owns:      []runtime.Object{&appsv1.Deployment{}, &v1alpha1.Channel{}},
		Reconciler: &reconciler{
			recorder: mgr.GetRecorder(controllerAgentName),
		},
	}

	return p.ProvideController(mgr)
}
