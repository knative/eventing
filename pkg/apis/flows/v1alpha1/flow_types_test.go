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

package v1alpha1

import (
	"fmt"
	"testing"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestFlowCondition_GetConditionNotFound(t *testing.T) {
	flow := Flow{}
	flow.Status.setCondition(&FlowCondition{Type: FlowConditionReady})
	if flow.Status.GetCondition(FlowConditionFeedReady) != nil {
		t.Fatalf("Got a non-nil for non-existent conditiontype")
	}

	flow2 := Flow{}
	if flow2.Status.GetCondition(FlowConditionFeedReady) != nil {
		t.Fatalf("Got a non-nil for non-existent conditiontype")
	}
}

func TestFlowCondition_GetCondition(t *testing.T) {
	testcases := []struct {
		name  string
		types []FlowConditionType
		get   FlowConditionType
		want  FlowConditionType
	}{
		{"FlowConditionReady", []FlowConditionType{FlowConditionReady}, FlowConditionReady, FlowConditionReady},
		{"FlowConditionFeedReady", []FlowConditionType{FlowConditionFeedReady, FlowConditionReady}, FlowConditionFeedReady, FlowConditionFeedReady},
		{"FlowConditionChannelReady", []FlowConditionType{FlowConditionChannelReady, FlowConditionReady}, FlowConditionChannelReady, FlowConditionChannelReady},
		{"FlowConditionSubscriptionReady", []FlowConditionType{FlowConditionSubscriptionReady}, FlowConditionSubscriptionReady, FlowConditionSubscriptionReady},
	}

	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "Flow", tc.name)
		t.Run(testName, func(t *testing.T) {
			flow := Flow{}
			for _, t := range tc.types {
				c := &FlowCondition{Type: t}
				flow.Status.setCondition(c)
			}

			if want, got := tc.want, flow.Status.GetCondition(tc.get).Type; want != got {
				t.Fatalf("Failed to get expected condition. \nwant:\t%#v\ngot:\t%#v", want, got)
			}
		})
	}
}

func TestFlowCondition_setCondition(t *testing.T) {
	testcases := []struct {
		name  string
		types []FlowConditionType
		want  int
	}{
		{"One", []FlowConditionType{FlowConditionReady}, 1},
		{"Two", []FlowConditionType{FlowConditionReady, FlowConditionFeedReady}, 2},
		{"Replace", []FlowConditionType{FlowConditionReady, FlowConditionReady}, 1},
		{"Invalid", []FlowConditionType{""}, 0},
	}

	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "Flow", tc.name)
		t.Run(testName, func(t *testing.T) {
			flow := Flow{}
			for _, t := range tc.types {
				c := &FlowCondition{Type: t}
				flow.Status.setCondition(c)
			}
			if want, got := tc.want, len(flow.Status.Conditions); want != got {
				t.Fatalf("Failed to return expected number of conditions. \nwant:\t%#v\ngot:\t%#v", want, got)
			}
		})
	}
}

func TestFlowCondition_RemoveCondition(t *testing.T) {
	testcases := []struct {
		name   string
		set    []FlowConditionType
		remove []FlowConditionType
		want   int
	}{
		{"RemoveOnlyOne", []FlowConditionType{FlowConditionReady}, []FlowConditionType{FlowConditionReady}, 0},
		{"RemoveOne", []FlowConditionType{FlowConditionReady, FlowConditionChannelReady}, []FlowConditionType{FlowConditionReady}, 1},
		{"RemoveNonExistent", []FlowConditionType{FlowConditionReady}, []FlowConditionType{FlowConditionSubscriptionReady}, 1},
		{"Invalid", []FlowConditionType{FlowConditionReady}, []FlowConditionType{""}, 1},
	}

	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "Flow", tc.name)
		t.Run(testName, func(t *testing.T) {
			flow := Flow{}
			for _, t := range tc.set {
				c := &FlowCondition{Type: t}
				flow.Status.setCondition(c)
			}
			for _, t := range tc.remove {
				flow.Status.removeCondition(t)
			}

			if want, got := tc.want, len(flow.Status.Conditions); want != got {
				t.Fatalf("Failed to return expected number of conditions. \nwant:\t%#v\ngot:\t%#v", want, got)
			}
		})
	}
}

func TestFlowCondition_IsReady(t *testing.T) {
	testcases := []struct {
		name string
		set  []FlowCondition
		want bool
	}{
		{"FlowConditionFeedReady", []FlowCondition{{
			Type:   FlowConditionFeedReady,
			Status: corev1.ConditionTrue,
		}},
			false},
		{"FlowConditionFeedAndChannelReady", []FlowCondition{{
			Type:   FlowConditionFeedReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   FlowConditionChannelReady,
			Status: corev1.ConditionTrue,
		}},
			false},
		{"FlowConditionFeedReadyChannelNotReady", []FlowCondition{{
			Type:   FlowConditionFeedReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   FlowConditionChannelReady,
			Status: corev1.ConditionFalse,
		}},
			false},
		{"FlowConditionFeedNotReadyChannelReady", []FlowCondition{{
			Type:   FlowConditionFeedReady,
			Status: corev1.ConditionFalse,
		}, {
			Type:   FlowConditionChannelReady,
			Status: corev1.ConditionTrue,
		}},
			false},
		{"FlowConditionFeedReadyChannelReadySubscriptionDispatching", []FlowCondition{{
			Type:   FlowConditionFeedReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   FlowConditionChannelReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   FlowConditionSubscriptionReady,
			Status: corev1.ConditionTrue,
		}},
			true},
	}

	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "Flow", tc.name)
		t.Run(testName, func(t *testing.T) {
			flow := Flow{}
			for _, c := range tc.set {
				flow.Status.setCondition(&c)
			}
			flow.Status.checkAndMarkReady()
			if want, got := tc.want, flow.Status.IsReady(); want != got {
				t.Fatalf("Failed IsReady check : \nwant:\t%#v\ngot:\t%#v", want, got)
			}
		})
	}
}

func TestFlowCondition_PropagateStatus(t *testing.T) {
	testcases := []struct {
		name                 string
		feedStatuses         []feedsv1alpha1.FeedStatus
		channelStatuses      []channelsv1alpha1.ChannelStatus
		subscriptionStatuses []channelsv1alpha1.SubscriptionStatus
		want                 bool
	}{
		{"NothingReady",
			[]feedsv1alpha1.FeedStatus{feedsv1alpha1.FeedStatus{}},
			[]channelsv1alpha1.ChannelStatus{channelsv1alpha1.ChannelStatus{}},
			[]channelsv1alpha1.SubscriptionStatus{channelsv1alpha1.SubscriptionStatus{}},
			false},
		{"FeedReady",
			[]feedsv1alpha1.FeedStatus{{
				Conditions: []feedsv1alpha1.FeedCondition{{
					Type:   feedsv1alpha1.FeedConditionReady,
					Status: corev1.ConditionTrue,
				}},
			}},
			[]channelsv1alpha1.ChannelStatus{},
			[]channelsv1alpha1.SubscriptionStatus{},
			false},
		{"ChannelReady",
			[]feedsv1alpha1.FeedStatus{},
			[]channelsv1alpha1.ChannelStatus{{
				Conditions: []channelsv1alpha1.ChannelCondition{{
					Type:   channelsv1alpha1.ChannelReady,
					Status: corev1.ConditionTrue,
				}},
				DomainInternal: "foobar-channel.default.svc.cluster.local",
			}},
			[]channelsv1alpha1.SubscriptionStatus{},
			false},
		{"SubscriptionReady",
			[]feedsv1alpha1.FeedStatus{},
			[]channelsv1alpha1.ChannelStatus{},
			[]channelsv1alpha1.SubscriptionStatus{{
				Conditions: []channelsv1alpha1.SubscriptionCondition{{
					Type:   channelsv1alpha1.SubscriptionDispatching,
					Status: corev1.ConditionTrue,
				}},
			}},
			false},
		{"AllNotReady",
			[]feedsv1alpha1.FeedStatus{{
				Conditions: []feedsv1alpha1.FeedCondition{{
					Type:   feedsv1alpha1.FeedConditionReady,
					Status: corev1.ConditionFalse,
				}},
			}},
			[]channelsv1alpha1.ChannelStatus{{
				Conditions: []channelsv1alpha1.ChannelCondition{{
					Type:   channelsv1alpha1.ChannelReady,
					Status: corev1.ConditionFalse,
				}},
				DomainInternal: "foobar-channel.default.svc.cluster.local",
			}},
			[]channelsv1alpha1.SubscriptionStatus{{
				Conditions: []channelsv1alpha1.SubscriptionCondition{{
					Type:   channelsv1alpha1.SubscriptionDispatching,
					Status: corev1.ConditionFalse,
				}},
			}},
			false},
		{"AllReady",
			[]feedsv1alpha1.FeedStatus{{
				Conditions: []feedsv1alpha1.FeedCondition{{
					Type:   feedsv1alpha1.FeedConditionReady,
					Status: corev1.ConditionTrue,
				}},
			}},
			[]channelsv1alpha1.ChannelStatus{{
				Conditions: []channelsv1alpha1.ChannelCondition{{
					Type:   channelsv1alpha1.ChannelReady,
					Status: corev1.ConditionTrue,
				}},
				DomainInternal: "foobar-channel.default.svc.cluster.local",
			}},
			[]channelsv1alpha1.SubscriptionStatus{{
				Conditions: []channelsv1alpha1.SubscriptionCondition{{
					Type:   channelsv1alpha1.SubscriptionDispatching,
					Status: corev1.ConditionTrue,
				}},
			}},
			true},
	}
	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "Flow", tc.name)
		t.Run(testName, func(t *testing.T) {
			flow := Flow{}
			for _, fs := range tc.feedStatuses {
				flow.Status.PropagateFeedStatus(fs)
			}
			for _, cs := range tc.channelStatuses {
				flow.Status.PropagateChannelStatus(cs)
			}
			for _, ss := range tc.subscriptionStatuses {
				flow.Status.PropagateSubscriptionStatus(ss)
			}
			if want, got := tc.want, flow.Status.IsReady(); want != got {
				t.Fatalf("Failed IsReady check : \nwant:\t%#v\ngot:\t%#v", want, got)
			}
		})
	}
}
