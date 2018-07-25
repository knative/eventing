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

package flow

import (
	//	"encoding/base64"
	//	"encoding/json"
	//	"fmt"
	"testing"

	//	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	/*
		"github.com/knative/eventing/pkg/controller/feed/resources"
		"github.com/knative/eventing/pkg/sources"
		batchv1 "k8s.io/api/batch/v1"
	*/)

/*
TODO
- initial: feed with job deadline exceeded
  reconciled: feed failure, job exists, finalizer
*/

var (
	trueVal  = true
	falseVal = false
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()
)

const (
	targetDNS = "myservice.mynamespace.svc.cluster.local"
)

func init() {
	// Add types to scheme
	feedsv1alpha1.AddToScheme(scheme.Scheme)
	servingv1alpha1.AddToScheme(scheme.Scheme)
}

var testCases = []controllertesting.TestCase{
	{
		Name:         "new feed: adds status, finalizer, creates job",
		InitialState: []runtime.Object{
			//			getEventSource(),
			//			getEventType(),
			//			getNewFeed(),
		},
		ReconcileKey: "test/test-feed",
		WantPresent:  []runtime.Object{
			//			getStartInProgressFeed(),
			//			getNewStartJob(),
			//TODO job created event
		},
	},
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		r := &reconciler{
			client:   tc.GetClient(),
			recorder: recorder,
		}
		t.Run(tc.Name, tc.Runner(t, r, r.client))
	}
}
