/*
Copyright 2019 The Knative Authors

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

package resources

import (
	"fmt"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MakeEventType creates the in-memory representation of the EventType for the specified CronJobSource.
func MakeEventType(src *v1alpha1.CronJobSource) *eventingv1alpha1.EventType {
	return &eventingv1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", utils.ToDNS1123Subdomain(v1alpha1.CronJobEventType)),
			Labels:       Labels(src.Name),
			Namespace:    src.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(src, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "CronJobSource",
				}),
			},
		},
		Spec: eventingv1alpha1.EventTypeSpec{
			Type:        v1alpha1.CronJobEventType,
			Source:      v1alpha1.CronJobEventSource,
			Broker:      src.Spec.Sink.Name,
			Description: src.Name,
		},
	}
}
