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

package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/Shopify/sarama"
	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/contrib/kafka/pkg/controller"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	util "github.com/knative/eventing/pkg/provisioners"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	_ "github.com/knative/pkg/system/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	channelName                   = "test-channel"
	clusterChannelProvisionerName = "kafka"
	testNS                        = "test-namespace"
	topicPrefix                   = "knative-eventing-channel"
	testUID                       = "test-uid"
	argumentNumPartitions         = "NumPartitions"
	argumentReplicationFactor     = "ReplicationFactor"
)

var (
	truePointer = true

	deletedTs = metav1.Now().Rfc3339Copy()

	// serviceAddress is the address of the K8s Service. It uses a GeneratedName and the fake client
	// does not fill in Name, so the name is the empty string.
	serviceAddress = fmt.Sprintf("%s.%s.svc.%s", "", testNS, utils.GetClusterDomainName())

	// map of events to set test cases' expectations easier
	events = map[string]corev1.Event{
		dispatcherReconcileFailed: {Reason: dispatcherReconcileFailed, Type: corev1.EventTypeWarning},
	}
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
	istiov1alpha3.AddToScheme(scheme.Scheme)
}

var mockFetchError = controllertesting.Mocks{
	MockGets: []controllertesting.MockGet{
		func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*eventingv1alpha1.Channel); ok {
				err := fmt.Errorf("error fetching")
				return controllertesting.Handled, err
			}
			return controllertesting.Unhandled, nil
		},
	},
}

type mockClusterAdmin struct {
	mockCreateTopicFunc func(topic string, detail *sarama.TopicDetail, validateOnly bool) error
	mockDeleteTopicFunc func(topic string) error
}

func (ca *mockClusterAdmin) CreateTopic(topic string, detail *sarama.TopicDetail, validateOnly bool) error {
	if ca.mockCreateTopicFunc != nil {
		return ca.mockCreateTopicFunc(topic, detail, validateOnly)
	}
	return nil
}

func (ca *mockClusterAdmin) Close() error {
	return nil
}

func (ca *mockClusterAdmin) DeleteTopic(topic string) error {
	if ca.mockDeleteTopicFunc != nil {
		return ca.mockDeleteTopicFunc(topic)
	}
	return nil
}

func (ca *mockClusterAdmin) CreatePartitions(topic string, count int32, assignment [][]int32, validateOnly bool) error {
	return nil
}

func (ca *mockClusterAdmin) DeleteRecords(topic string, partitionOffsets map[int32]int64) error {
	return nil
}

func (ca *mockClusterAdmin) DescribeConfig(resource sarama.ConfigResource) ([]sarama.ConfigEntry, error) {
	return nil, nil
}

func (ca *mockClusterAdmin) AlterConfig(resourceType sarama.ConfigResourceType, name string, entries map[string]*string, validateOnly bool) error {
	return nil
}

func (ca *mockClusterAdmin) CreateACL(resource sarama.Resource, acl sarama.Acl) error {
	return nil
}

func (ca *mockClusterAdmin) ListAcls(filter sarama.AclFilter) ([]sarama.ResourceAcls, error) {
	return nil, nil
}

func (ca *mockClusterAdmin) DeleteACL(filter sarama.AclFilter, validateOnly bool) ([]sarama.MatchingAcl, error) {
	return nil, nil
}

var testCases = []controllertesting.TestCase{
	{
		Name: "new channel with valid provisioner: adds finalizer",
		InitialState: []runtime.Object{
			getNewClusterChannelProvisioner(clusterChannelProvisionerName, true),
			getNewChannel(channelName, clusterChannelProvisionerName),
			makeVirtualService(),
		},
		WantResult: reconcile.Result{
			Requeue: true,
		},
		WantPresent: []runtime.Object{
			getNewChannelWithStatusAndFinalizer(channelName, clusterChannelProvisionerName),
		},
	},
	{
		Name: "new channel with valid provisioner and finalizer: adds provisioned status",
		InitialState: []runtime.Object{
			getNewClusterChannelProvisioner(clusterChannelProvisionerName, true),
			getNewChannelWithStatusAndFinalizer(channelName, clusterChannelProvisionerName),
			makeVirtualService(),
		},
		WantPresent: []runtime.Object{
			getNewChannelProvisionedStatus(channelName, clusterChannelProvisionerName),
		},
	},
	{
		Name: "new channel with provisioner not ready: error",
		InitialState: []runtime.Object{
			getNewClusterChannelProvisioner(clusterChannelProvisionerName, false),
			getNewChannel(channelName, clusterChannelProvisionerName),
		},
		WantErrMsg: "ClusterChannelProvisioner " + clusterChannelProvisionerName + " is not ready",
		WantPresent: []runtime.Object{
			getNewChannelNotProvisionedStatus(channelName, clusterChannelProvisionerName,
				"ClusterChannelProvisioner "+clusterChannelProvisionerName+" is not ready"),
		},
		WantEvent: []corev1.Event{
			events[dispatcherReconcileFailed],
		},
	},
	{
		Name: "new channel with missing provisioner: error",
		InitialState: []runtime.Object{
			getNewChannel(channelName, clusterChannelProvisionerName),
		},
		WantErrMsg: "clusterchannelprovisioners.eventing.knative.dev \"" + clusterChannelProvisionerName + "\" not found",
	},
	{
		Name: "new channel with provisioner not managed by this controller: skips channel",
		InitialState: []runtime.Object{
			getNewChannel(channelName, "not-our-provisioner"),
			getNewClusterChannelProvisioner("not-our-provisioner", true),
			getNewClusterChannelProvisioner(clusterChannelProvisionerName, true),
		},
		WantPresent: []runtime.Object{
			getNewChannel(channelName, "not-our-provisioner"),
		},
	},
	{
		Name: "new channel with missing provisioner reference: skips channel",
		InitialState: []runtime.Object{
			getNewChannelNoProvisioner(channelName),
		},
		WantPresent: []runtime.Object{
			getNewChannelNoProvisioner(channelName),
		},
	},
	{
		Name:         "channel not found",
		InitialState: []runtime.Object{},
		WantPresent:  []runtime.Object{},
	},
	{
		Name: "error fetching channel",
		InitialState: []runtime.Object{
			getNewClusterChannelProvisioner(clusterChannelProvisionerName, true),
			getNewChannel(channelName, clusterChannelProvisionerName),
		},
		Mocks:      mockFetchError,
		WantErrMsg: "error fetching",
		WantPresent: []runtime.Object{
			getNewClusterChannelProvisioner(clusterChannelProvisionerName, true),
			getNewChannel(channelName, clusterChannelProvisionerName),
		},
	},
	{
		Name: "deleted channel",
		InitialState: []runtime.Object{
			getNewClusterChannelProvisioner(clusterChannelProvisionerName, true),
			getNewChannelDeleted(channelName, clusterChannelProvisionerName),
		},
		WantPresent: []runtime.Object{},
	},
}

func TestAllCases(t *testing.T) {

	for _, tc := range testCases {
		tc.ReconcileKey = fmt.Sprintf("%s/%s", testNS, channelName)
		tc.IgnoreTimes = true

		c := tc.GetClient()
		recorder := tc.GetEventRecorder()
		logger := provisioners.NewProvisionerLoggerFromConfig(provisioners.NewLoggingConfig())
		r := &reconciler{
			client:            c,
			recorder:          recorder,
			logger:            logger.Desugar(),
			config:            getControllerConfig(),
			kafkaClusterAdmin: &mockClusterAdmin{},
		}
		t.Logf("Running test %s", tc.Name)
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func TestProvisionChannel(t *testing.T) {
	provisionTestCases := []struct {
		name            string
		c               *eventingv1alpha1.Channel
		wantTopicName   string
		wantTopicDetail *sarama.TopicDetail
		mockError       error
		wantError       string
	}{
		{
			name:          "provision with no channel arguments - uses default",
			c:             getNewChannel(channelName, clusterChannelProvisionerName),
			wantTopicName: fmt.Sprintf("%s.%s.%s", topicPrefix, testNS, channelName),
			wantTopicDetail: &sarama.TopicDetail{
				ReplicationFactor: 1,
				NumPartitions:     1,
			},
		},
		{
			name:          "provision with unknown channel arguments - uses default",
			c:             getNewChannelWithArgs(channelName, map[string]interface{}{"testing": "testing"}),
			wantTopicName: fmt.Sprintf("%s.%s.%s", topicPrefix, testNS, channelName),
			wantTopicDetail: &sarama.TopicDetail{
				ReplicationFactor: 1,
				NumPartitions:     1,
			},
		},
		{
			name:      "provision with invalid channel arguments - errors",
			c:         getNewChannelWithArgs(channelName, map[string]interface{}{argumentNumPartitions: "invalid"}),
			wantError: fmt.Sprintf("error unmarshalling arguments: json: cannot unmarshal string into Go struct field channelArgs.%s of type int32", argumentNumPartitions),
		},
		{
			name:      "provision with invalid channel arguments - errors",
			c:         getNewChannelWithArgs(channelName, map[string]interface{}{argumentReplicationFactor: "invalid"}),
			wantError: fmt.Sprintf("error unmarshalling arguments: json: cannot unmarshal string into Go struct field channelArgs.%s of type int16", argumentReplicationFactor),
		},
		{
			name:      "provision with nil channel arguments - errors",
			c:         getNewChannelWithArgs(channelName, map[string]interface{}{argumentNumPartitions: "nil"}),
			wantError: fmt.Sprintf("error unmarshalling arguments: json: cannot unmarshal string into Go struct field channelArgs.%s of type int32", argumentNumPartitions),
		},
		{
			name:      "provision with nil channel arguments - errors",
			c:         getNewChannelWithArgs(channelName, map[string]interface{}{argumentReplicationFactor: "nil"}),
			wantError: fmt.Sprintf("error unmarshalling arguments: json: cannot unmarshal string into Go struct field channelArgs.%s of type int16", argumentReplicationFactor),
		},
		{
			name: "provision with unmarshallable channel arguments - errors",
			c: func() *eventingv1alpha1.Channel {
				channel := getNewChannel(channelName, clusterChannelProvisionerName)
				channel.Spec.Arguments = &runtime.RawExtension{
					Raw: []byte("invalid"),
				}
				return channel
			}(),
			wantError: "error unmarshalling arguments: invalid character 'i' looking for beginning of value",
		},
		{
			name:          "provision with valid channel arguments",
			c:             getNewChannelWithArgs(channelName, map[string]interface{}{argumentNumPartitions: 2}),
			wantTopicName: fmt.Sprintf("%s.%s.%s", topicPrefix, testNS, channelName),
			wantTopicDetail: &sarama.TopicDetail{
				ReplicationFactor: 1,
				NumPartitions:     2,
			},
		},
		{
			name:          "provision but topic already exists - no error",
			c:             getNewChannelWithArgs(channelName, map[string]interface{}{argumentNumPartitions: 2}),
			wantTopicName: fmt.Sprintf("%s.%s.%s", topicPrefix, testNS, channelName),
			wantTopicDetail: &sarama.TopicDetail{
				ReplicationFactor: 1,
				NumPartitions:     2,
			},
			mockError: sarama.ErrTopicAlreadyExists,
		},
		{
			name:          "provision but error creating topic",
			c:             getNewChannelWithArgs(channelName, map[string]interface{}{argumentNumPartitions: 2}),
			wantTopicName: fmt.Sprintf("%s.%s.%s", topicPrefix, testNS, channelName),
			wantTopicDetail: &sarama.TopicDetail{
				ReplicationFactor: 1,
				NumPartitions:     2,
			},
			mockError: fmt.Errorf("unknown sarama error"),
			wantError: "unknown sarama error",
		}}

	for _, tc := range provisionTestCases {
		t.Logf("running test %s", tc.name)
		logger := provisioners.NewProvisionerLoggerFromConfig(provisioners.NewLoggingConfig())
		r := &reconciler{
			logger: logger.Desugar(),
		}
		kafkaClusterAdmin := &mockClusterAdmin{
			mockCreateTopicFunc: func(topic string, detail *sarama.TopicDetail, validateOnly bool) error {
				if topic != tc.wantTopicName {
					t.Errorf("expected topic name: %+v got: %+v", tc.wantTopicName, topic)
				}
				return tc.mockError
			}}
		err := r.provisionChannel(tc.c, kafkaClusterAdmin)
		var got string
		if err != nil {
			got = err.Error()
		}
		if diff := cmp.Diff(tc.wantError, got); diff != "" {
			t.Errorf("unexpected error (-want, +got) = %v", diff)
		}
	}
}

func TestDeprovisionChannel(t *testing.T) {
	deprovisionTestCases := []struct {
		name          string
		c             *eventingv1alpha1.Channel
		wantTopicName string
		mockError     error
		wantError     string
	}{
		{
			name:          "deprovision channel - unknown error",
			c:             getNewChannel(channelName, clusterChannelProvisionerName),
			wantTopicName: fmt.Sprintf("%s.%s.%s", topicPrefix, testNS, channelName),
			mockError:     fmt.Errorf("unknown sarama error"),
			wantError:     "unknown sarama error",
		},
		{
			name:          "deprovision channel - topic already deleted",
			c:             getNewChannel(channelName, clusterChannelProvisionerName),
			wantTopicName: fmt.Sprintf("%s.%s.%s", topicPrefix, testNS, channelName),
			mockError:     sarama.ErrUnknownTopicOrPartition,
		},
		{
			name:          "deprovision channel - success",
			c:             getNewChannel(channelName, clusterChannelProvisionerName),
			wantTopicName: fmt.Sprintf("%s.%s.%s", topicPrefix, testNS, channelName),
		}}

	for _, tc := range deprovisionTestCases {
		t.Logf("running test %s", tc.name)
		logger := provisioners.NewProvisionerLoggerFromConfig(provisioners.NewLoggingConfig())
		r := &reconciler{
			logger: logger.Desugar()}
		kafkaClusterAdmin := &mockClusterAdmin{
			mockDeleteTopicFunc: func(topic string) error {
				if topic != tc.wantTopicName {
					t.Errorf("expected topic name: %+v got: %+v", tc.wantTopicName, topic)
				}
				return tc.mockError
			}}

		err := r.deprovisionChannel(tc.c, kafkaClusterAdmin)
		var got string
		if err != nil {
			got = err.Error()
		}
		if diff := cmp.Diff(tc.wantError, got); diff != "" {
			t.Errorf("unexpected error (-want, +got) = %v", diff)
		}
	}
}

func getNewChannelNoProvisioner(name string) *eventingv1alpha1.Channel {
	channel := &eventingv1alpha1.Channel{
		TypeMeta:   channelType(),
		ObjectMeta: om(testNS, name),
		Spec:       eventingv1alpha1.ChannelSpec{},
	}
	// selflink is not filled in when we create the object, so clear it
	channel.ObjectMeta.SelfLink = ""
	return channel
}

func getNewChannel(name, provisioner string) *eventingv1alpha1.Channel {
	channel := &eventingv1alpha1.Channel{
		TypeMeta:   channelType(),
		ObjectMeta: om(testNS, name),
		Spec: eventingv1alpha1.ChannelSpec{
			Provisioner: &corev1.ObjectReference{
				Name:       provisioner,
				Kind:       "ClusterChannelProvisioner",
				APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	channel.ObjectMeta.SelfLink = ""
	return channel
}

func getNewChannelWithFinalizer(name, provisioner string) *eventingv1alpha1.Channel {
	c := getNewChannel(name, provisioner)
	util.AddFinalizer(c, finalizerName)
	return c
}

func getNewChannelWithStatusAndFinalizer(name, provisioner string) *eventingv1alpha1.Channel {
	c := getNewChannelWithFinalizer(name, provisioner)
	c.Status.InitializeConditions()
	return c
}

func getNewChannelWithArgs(name string, args map[string]interface{}) *eventingv1alpha1.Channel {
	c := getNewChannelNoProvisioner(name)
	bytes, _ := json.Marshal(args)
	c.Spec.Arguments = &runtime.RawExtension{
		Raw: bytes,
	}
	return c
}

func getNewChannelProvisionedStatus(name, provisioner string) *eventingv1alpha1.Channel {
	c := getNewChannel(name, provisioner)
	c.Status.InitializeConditions()
	c.Status.SetAddress(serviceAddress)
	c.Status.MarkProvisioned()
	c.Finalizers = []string{finalizerName}
	return c
}

func getNewChannelDeleted(name, provisioner string) *eventingv1alpha1.Channel {
	c := getNewChannelProvisionedStatus(name, provisioner)
	c.DeletionTimestamp = &deletedTs
	return c
}

func getNewChannelNotProvisionedStatus(name, provisioner, msg string) *eventingv1alpha1.Channel {
	c := getNewChannel(name, provisioner)
	c.Status.InitializeConditions()
	c.Status.MarkNotProvisioned("NotProvisioned", msg)
	return c
}

func channelType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Channel",
	}
}

func getNewClusterChannelProvisioner(name string, isReady bool) *eventingv1alpha1.ClusterChannelProvisioner {
	var condStatus corev1.ConditionStatus
	if isReady {
		condStatus = corev1.ConditionTrue
	} else {
		condStatus = corev1.ConditionFalse
	}
	clusterChannelProvisioner := &eventingv1alpha1.ClusterChannelProvisioner{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ClusterChannelProvisioner",
		},
		ObjectMeta: om("", name),
		Spec:       eventingv1alpha1.ClusterChannelProvisionerSpec{},
		Status: eventingv1alpha1.ClusterChannelProvisionerStatus{
			Conditions: []duckv1alpha1.Condition{
				{
					Type:   eventingv1alpha1.ClusterChannelProvisionerConditionReady,
					Status: condStatus,
				},
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	clusterChannelProvisioner.ObjectMeta.SelfLink = ""
	return clusterChannelProvisioner
}

func makeVirtualService() *istiov1alpha3.VirtualService {
	return &istiov1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: istiov1alpha3.SchemeGroupVersion.String(),
			Kind:       "VirtualService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-channel", testNS),
			Namespace: testNS,
			Labels: map[string]string{
				"channel":     channelName,
				"provisioner": clusterChannelProvisionerName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "Channel",
					Name:               channelName,
					UID:                testUID,
					Controller:         &truePointer,
					BlockOwnerDeletion: &truePointer,
				},
			},
		},
		Spec: istiov1alpha3.VirtualServiceSpec{
			Hosts: []string{
				serviceAddress,
				fmt.Sprintf("%s.%s.channels.%s", channelName, testNS, utils.GetClusterDomainName()),
			},
			Http: []istiov1alpha3.HTTPRoute{{
				Rewrite: &istiov1alpha3.HTTPRewrite{
					Authority: fmt.Sprintf("%s.%s.channels.%s", channelName, testNS, utils.GetClusterDomainName()),
				},
				Route: []istiov1alpha3.DestinationWeight{{
					Destination: istiov1alpha3.Destination{
						Host: "kafka-provisioner.knative-testing.svc." + utils.GetClusterDomainName(),
						Port: istiov1alpha3.PortSelector{
							Number: util.PortNumber,
						},
					}},
				}},
			},
		},
	}
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}

func getControllerConfig() *controller.KafkaProvisionerConfig {
	return &controller.KafkaProvisionerConfig{
		Brokers: []string{"test-broker"},
	}
}
