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

package psbinding

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gobuffalo/flect"
	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

// ReconcilerOption is a function to modify the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithSelector specifies the selector for the webhook.
func WithSelector(s metav1.LabelSelector) ReconcilerOption {
	return func(r *Reconciler) {
		r.selector = s
	}
}

func NewReconciler(
	name, path, secretName string,
	client kubernetes.Interface,
	mwhLister admissionlisters.MutatingWebhookConfigurationLister,
	secretLister corelisters.SecretLister,
	withContext BindableContext,
	options ...ReconcilerOption,
) *Reconciler {
	r := &Reconciler{
		Name:        name,
		HandlerPath: path,
		SecretName:  secretName,

		// This is the user-provided context-decorator, which allows
		// them to infuse the context passed to Do/Undo.
		WithContext: withContext,

		Client:       client,
		MWHLister:    mwhLister,
		SecretLister: secretLister,
		selector:     ExclusionSelector, // Use ExclusionSelector by default.
	}

	// Apply options.
	for _, opt := range options {
		opt(r)
	}

	return r
}

// Reconciler implements an AdmissionController for altering PodSpecable
// resources that are the subject of a particular type of Binding.
// The two key methods are:
//  1. reconcileMutatingWebhook: which enumerates all of the Bindings and
//     compiles a list of resource types that should be intercepted by our
//     webhook.  It also builds an index that can be used to efficiently
//     handle Admit requests.
//  2. Admit: which leverages the index built by the Reconciler to apply
//     mutations to resources.
type Reconciler struct {
	pkgreconciler.LeaderAwareFuncs

	Name        string
	HandlerPath string
	SecretName  string

	Client       kubernetes.Interface
	MWHLister    admissionlisters.MutatingWebhookConfigurationLister
	SecretLister corelisters.SecretLister
	ListAll      ListAll

	// WithContext is a callback that infuses the context supplied to
	// Do/Undo with additional context to enable them to complete their
	// respective tasks.
	WithContext BindableContext

	selector metav1.LabelSelector

	index index
}

var _ controller.Reconciler = (*Reconciler)(nil)
var _ pkgreconciler.LeaderAware = (*Reconciler)(nil)
var _ webhook.AdmissionController = (*Reconciler)(nil)

// We need to specifically exclude our deployment(s) from consideration, but this provides a way
// of excluding other things as well.
var (
	ExclusionSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      duck.BindingExcludeLabel,
			Operator: metav1.LabelSelectorOpNotIn,
			Values:   []string{"true"},
		}},
		// TODO(mattmoor): Consider also having a GVR-based one, e.g.
		//    foobindings.blah.knative.dev/exclude: "true"
	}
	InclusionSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      duck.BindingIncludeLabel,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{"true"},
		}},
		// TODO(mattmoor): Consider also having a GVR-based one, e.g.
		//    foobindings.blah.knative.dev/include: "true"
	}
)

// Reconcile implements controller.Reconciler
func (ac *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Look up the webhook secret, and fetch the CA cert bundle.
	secret, err := ac.SecretLister.Secrets(system.Namespace()).Get(ac.SecretName)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error fetching secret", zap.Error(err))
		return err
	}
	caCert, ok := secret.Data[certresources.CACert]
	if !ok {
		return fmt.Errorf("secret %q is missing %q key", ac.SecretName, certresources.CACert)
	}

	// Reconcile the webhook configuration.
	return ac.reconcileMutatingWebhook(ctx, caCert)
}

// Path implements AdmissionController
func (ac *Reconciler) Path() string {
	return ac.HandlerPath
}

// Admit implements AdmissionController
func (ac *Reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	switch request.Operation {
	case admissionv1.Create, admissionv1.Update:
	default:
		logging.FromContext(ctx).Info("Unhandled webhook operation, letting it through ", request.Operation)
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	orig := &duckv1.WithPod{}
	if err := json.Unmarshal(request.Object.Raw, orig); err != nil {
		return webhook.MakeErrorStatus("unable to decode object: %v", err)
	}

	// Look up the Bindables for this resource.
	fbs := ac.index.lookUp(exactKey{
		Group:     request.Kind.Group,
		Kind:      request.Kind.Kind,
		Namespace: request.Namespace,
		Name:      orig.Name},
		labels.Set(orig.Labels))
	if len(fbs) == 0 {
		// This doesn't apply!
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	// Copy the subject state.
	mutated := orig.DeepCopy()

	// Apply the Bindables to the copy of the subject state. If conflicts occur, for example because multiple Bindables
	// make incompatible changes, the reconciler will attempt to correct the state later.
	for _, fb := range fbs {
		var bindingContext context.Context
		// Callback into the user's code to setup the context with additional
		// information needed to perform the mutation.
		if ac.WithContext != nil {
			var err error
			bindingContext, err = ac.WithContext(ctx, fb)
			if err != nil {
				return webhook.MakeErrorStatus("unable to setup binding context: %v", err)
			}
		} else {
			bindingContext = ctx
		}

		// Mutate the copy of the subject state according to the deletion state of the Bindable.
		if fb.GetDeletionTimestamp() != nil {
			fb.Undo(bindingContext, mutated)
		} else {
			fb.Do(bindingContext, mutated)
		}
	}

	// Synthesize a patch from the changes and return it in our AdmissionResponse
	patchBytes, err := duck.CreateBytePatch(orig, mutated)
	if err != nil {
		return webhook.MakeErrorStatus("unable to create patch with binding: %v", err)
	}
	return &admissionv1.AdmissionResponse{
		Patch:   patchBytes,
		Allowed: true,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (ac *Reconciler) reconcileMutatingWebhook(ctx context.Context, caCert []byte) error {
	// Build a deduplicated list of all of the GVKs we see.
	gks := map[schema.GroupKind]sets.String{}

	// When reconciling the webhook, enumerate all of the bindings, so that
	// we can index them to efficiently respond to webhook requests.
	fbs, err := ac.ListAll()
	if err != nil {
		return err
	}

	ib := newIndexBuilder()
	for _, fb := range fbs {
		ref := fb.GetSubject()
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return err
		}
		gk := schema.GroupKind{
			Group: gv.Group,
			Kind:  ref.Kind,
		}
		set := gks[gk]
		if set == nil {
			set = make(sets.String, 1)
		}
		set.Insert(gv.Version)
		gks[gk] = set

		if ref.Name != "" {
			ib.associate(exactKey{
				Group:     gk.Group,
				Kind:      gk.Kind,
				Namespace: ref.Namespace,
				Name:      ref.Name},
				fb)
		} else {
			selector, err := metav1.LabelSelectorAsSelector(ref.Selector)
			if err != nil {
				return err
			}
			ib.associateSelection(inexactKey{
				Group:     gk.Group,
				Kind:      gk.Kind,
				Namespace: ref.Namespace},
				selector, fb)
		}
	}

	// Update the index.
	ib.build(&ac.index)

	// After we've updated our indices, bail out unless we are the leader.
	// Only the leader should be mutating the webhook.
	if !ac.IsLeaderFor(sentinel) {
		// We don't use controller.NewSkipKey here because we did do
		// some amount of processing and the timing information may be
		// useful.
		return nil
	}

	rules := make([]admissionregistrationv1.RuleWithOperations, 0, len(gks))
	for gk, versions := range gks {
		plural := strings.ToLower(flect.Pluralize(gk.Kind))

		rules = append(rules, admissionregistrationv1.RuleWithOperations{
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
				admissionregistrationv1.Update,
			},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{gk.Group},
				APIVersions: versions.List(),
				Resources:   []string{plural + "/*"},
			},
		})
	}

	// Sort the rules by Group, Version, Kind so that things are deterministically ordered.
	sort.Slice(rules, func(i, j int) bool {
		lhs, rhs := rules[i], rules[j]
		if lhs.APIGroups[0] != rhs.APIGroups[0] {
			return lhs.APIGroups[0] < rhs.APIGroups[0]
		}
		if lhs.APIVersions[0] != rhs.APIVersions[0] {
			return lhs.APIVersions[0] < rhs.APIVersions[0]
		}
		return lhs.Resources[0] < rhs.Resources[0]
	})

	configuredWebhook, err := ac.MWHLister.Get(ac.Name)
	if err != nil {
		return fmt.Errorf("error retrieving webhook: %w", err)
	}
	current := configuredWebhook.DeepCopy()

	// Use the "Equivalent" match policy so that we don't need to enumerate versions for same-types.
	// This is only supported by 1.15+ clusters.
	matchPolicy := admissionregistrationv1.Equivalent

	for i, wh := range current.Webhooks {
		if wh.Name != current.Name {
			continue
		}
		cur := &current.Webhooks[i]
		selector := webhook.EnsureLabelSelectorExpressions(cur.NamespaceSelector, &ac.selector)

		cur.MatchPolicy = &matchPolicy
		cur.Rules = rules
		cur.NamespaceSelector = selector
		cur.ObjectSelector = selector // 1.15+ only
		cur.ClientConfig.CABundle = caCert
		if cur.ClientConfig.Service == nil {
			return fmt.Errorf("missing service reference for webhook: %s", wh.Name)
		}
		cur.ClientConfig.Service.Path = ptr.String(ac.Path())
	}

	if ok := equality.Semantic.DeepEqual(configuredWebhook, current); !ok {
		logging.FromContext(ctx).Info("Updating webhook")
		mwhclient := ac.Client.AdmissionregistrationV1().MutatingWebhookConfigurations()
		if _, err := mwhclient.Update(ctx, current, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update webhook: %w", err)
		}
	} else {
		logging.FromContext(ctx).Info("Webhook is valid")
	}
	return nil
}
