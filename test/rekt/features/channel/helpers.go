/*
Copyright 2021 The Knative Authors

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

package channel

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	authv1 "k8s.io/api/authorization/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"knative.dev/pkg/apis/duck"
	apiextensionsclient "knative.dev/pkg/client/injection/apiextensions/client"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/state"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingclientsetv1 "knative.dev/eventing/pkg/client/clientset/versioned/typed/messaging/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
)

func todo(ctx context.Context, t feature.T) {
	t.Log("TODO, Implement this.")
}

type EventingClient struct {
	Channels    messagingclientsetv1.ChannelInterface
	ChannelImpl dynamic.ResourceInterface
}

func Client(ctx context.Context) *EventingClient {
	env := environment.FromContext(ctx)

	mc := eventingclient.Get(ctx).MessagingV1()
	dc := dynamicclient.Get(ctx)

	return &EventingClient{
		Channels:    mc.Channels(env.Namespace()),
		ChannelImpl: dc.Resource(channel_impl.GVR()).Namespace(env.Namespace()),
	}
}

const (
	ChannelableNameKey = "channelableName"
)

func setChannelableName(name string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, ChannelableNameKey, name)
	}
}

func getChannelable(ctx context.Context, t feature.T) *duckv1.Channelable {
	name := state.GetStringOrFail(ctx, t, ChannelableNameKey)
	return getChannelableFromName(name, ctx, t)
}

func getChannelableFromName(name string, ctx context.Context, t feature.T) *duckv1.Channelable {
	c := Client(ctx)

	obj, err := c.ChannelImpl.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get ChannelImpl, %v", err)
	}

	channel := &duckv1.Channelable{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, channel); err != nil {
		t.Fatalf("Failed to convert channelImpl to Channelable: %v", err)
	}

	channel.ResourceVersion = channel_impl.GVR().Version
	channel.APIVersion = channel_impl.GVR().GroupVersion().String()

	return channel
}

//nolint:unused
func patchChannelable(ctx context.Context, t feature.T, before, after *duckv1.Channelable) {
	patch, err := duck.CreateMergePatch(before, after)
	if err != nil {
		t.Fatalf("Failed to create merge patch: %v", err)
	}
	// If there is nothing to patch, we are good, just return.
	// Empty patch is {}, hence we check for that.
	if len(patch) <= 2 {
		return
	}

	_, err = Client(ctx).ChannelImpl.Patch(ctx, before.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		t.Fatal("Failed to patch the ChannelImpl", zap.Error(err), zap.Any("patch", patch))
	}
}

func crdOfChannel(ctx context.Context, t feature.T) *apiextv1.CustomResourceDefinition {
	gvr := channel_impl.GVR()
	name := strings.Join([]string{gvr.Resource, gvr.Group}, ".")

	crd, err := apiextensionsclient.Get(ctx).ApiextensionsV1().CustomResourceDefinitions().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	return crd
}

func crdOfChannelIsNamespaced(ctx context.Context, t feature.T) {
	crd := crdOfChannel(ctx, t)

	if crd.Spec.Scope != apiextv1.NamespaceScoped {
		t.Logf("%q CRD is not namespaced", crd.Name)
		t.Fail()
	}
}

func crdOfChannelIsLabeled(key, want string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		crd := crdOfChannel(ctx, t)

		if got, found := crd.Labels[key]; !found {
			t.Logf("%q CRD does not have label %q", crd.Name, key)
			t.Fail()
		} else if got != want {
			t.Logf("%q CRD label %q expected to be %s, got %s", crd.Name, key, want, got)
			t.Fail()
		}
	}
}

func crdOfChannelHasCategory(want string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		crd := crdOfChannel(ctx, t)

		for _, got := range crd.Spec.Names.Categories {
			if got == want {
				// Success!
				return
			}
		}
		t.Logf("%q CRD does not have Category %q", crd.Name, want)
	}
}

// TODO: move this to framework?
func ServiceAccountSubjectAccessReviewAllowedOrFail(ctx context.Context, t feature.T, gvr schema.GroupVersionResource, subresource string, saName string, verb string) {
	env := environment.FromContext(ctx)
	kube := kubeclient.Get(ctx)

	r, err := kube.AuthorizationV1().SubjectAccessReviews().Create(ctx, &authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User: fmt.Sprintf("system:serviceaccount:%s:%s", env.Namespace(), saName),
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:        verb,
				Group:       gvr.Group,
				Version:     gvr.Version,
				Resource:    gvr.Resource,
				Subresource: subresource,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Error while checking if %q is not allowed on %s.%s/%s subresource:%q. err: %q", verb, gvr.Resource, gvr.Group, gvr.Version, subresource, err)
	}
	if !r.Status.Allowed {
		t.Fatalf("Operation %q is not allowed on %s.%s/%s subresource:%q", verb, gvr.Resource, gvr.Group, gvr.Version, subresource)
	}
}
