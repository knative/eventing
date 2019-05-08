/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/webhook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Broker struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Broker.
	Spec BrokerSpec `json:"spec,omitempty"`

	// Status represents the current state of the Broker. This data may be out of
	// date.
	// +optional
	Status BrokerStatus `json:"status,omitempty"`
}

// Check that Broker can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*Broker)(nil)
var _ apis.Defaultable = (*Broker)(nil)
var _ apis.Immutable = (*Broker)(nil)
var _ runtime.Object = (*Broker)(nil)
var _ webhook.GenericCRD = (*Broker)(nil)

type BrokerSpec struct {
	// ChannelTemplate, if specified will be used to create all the Channels used internally by the
	// Broker. Only Provisioner and Arguments may be specified. If left unspecified, the default
	// Channel for the namespace will be used.
	//
	// +optional
	ChannelTemplate *ChannelSpec `json:"channelTemplate,omitempty"`
}

// BrokerStatus represents the current state of a Broker.
type BrokerStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// Broker is Addressable. It currently exposes the endpoint as a
	// fully-qualified DNS name which will distribute traffic over the
	// provided targets from inside the cluster.
	//
	// It generally has the form {broker}-router.{namespace}.svc.{cluster domain name}
	Address duckv1alpha1.Addressable `json:"address,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BrokerList is a collection of Brokers.
type BrokerList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Broker `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for Brokers
func (t *Broker) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Broker")
}
