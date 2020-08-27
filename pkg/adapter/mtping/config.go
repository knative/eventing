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

package mtping

import (
	corev1 "k8s.io/api/core/v1"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/sources/v1beta1"
)

type PingConfig struct {
	corev1.ObjectReference `json:",inline"`

	// Schedule is the cronjob schedule. Defaults to `* * * * *`.
	Schedule string `json:"schedule"`

	// JsonData is json encoded data used as the body of the event posted to
	// the sink. Default is empty. If set, datacontenttype will also be set
	// to "application/json".
	// +optional
	JsonData string `json:"jsonData,omitempty"`

	// Extensions specify what attribute are added or overridden on the
	// outbound event. Each `Extensions` key-value pair are set on the event as
	// an attribute extension independently.
	// +optional
	Extensions map[string]string `json:"extensions,omitempty"`

	// SinkURI is the current active sink URI that has been configured for the
	// Source.
	SinkURI string `json:"sinkUri,omitempty"`
}

type PingConfigs map[string]PingConfig

// Project creates a PingConfig for the given source
func Project(i interface{}) interface{} {
	obj := i.(*v1beta1.PingSource)

	if scope, ok := obj.Annotations[eventing.ScopeAnnotationKey]; ok && scope != eventing.ScopeCluster {
		return nil
	}

	cfg := &PingConfig{
		ObjectReference: corev1.ObjectReference{
			Name:            obj.Name,
			Namespace:       obj.Namespace,
			UID:             obj.UID,
			ResourceVersion: obj.ResourceVersion,
		},
		Schedule: obj.Spec.Schedule,
		JsonData: obj.Spec.JsonData,
		SinkURI:  obj.Status.SinkURI.String(),
	}
	if obj.Spec.CloudEventOverrides != nil {
		cfg.Extensions = obj.Spec.CloudEventOverrides.Extensions
	}
	return cfg
}
