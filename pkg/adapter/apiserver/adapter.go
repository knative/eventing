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

package apiserver

import (
	"context"
	"fmt"
	"github.com/knative/pkg/apis/duck"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/knative/eventing/pkg/reconciler"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "ApiServerSource"

	controllerAgentName = "apiserver-source-adapter-controller"

	addEventType    = "dev.knative.apiserver.object.add"
	updateEventType = "dev.knative.apiserver.object.update"
	deleteEventType = "dev.knative.apiserver.object.delete"
)

type KubernetesEvent struct {
	Object    *duckv1alpha1.KResource `json:"obj,omitempty"`
	NewObject *duckv1alpha1.KResource `json:"newObj,omitempty"`
	OldObject *duckv1alpha1.KResource `json:"oldObj,omitempty"`
}

// Creates a URI of the form found in object metadata selfLinks
// Format looks like: /apis/feeds.knative.dev/v1alpha1/namespaces/default/feeds/k8s-events-example
// KNOWN ISSUES:
// * ObjectReference.APIVersion has no version information (e.g. serving.knative.dev rather than serving.knative.dev/v1alpha1)
// * ObjectReference does not have enough information to create the pluaralized list type (e.g. "revisions" from kind: Revision)
//
// Track these issues at https://github.com/kubernetes/kubernetes/issues/66313
// We could possibly work around this by adding a lister for the resources referenced by these events.
func createSelfLink(o corev1.ObjectReference) string {
	collectionNameHack := strings.ToLower(o.Kind) + "s"
	versionNameHack := o.APIVersion

	// Core API types don't have a separate package name and only have a version string (e.g. /apis/v1/namespaces/default/pods/myPod)
	// To avoid weird looking strings like "v1/versionUnknown" we'll sniff for a "." in the version
	if strings.Contains(versionNameHack, ".") && !strings.Contains(versionNameHack, "/") {
		versionNameHack = versionNameHack + "/versionUnknown"
	}
	return fmt.Sprintf("/apis/%s/namespaces/%s/%s/%s", versionNameHack, o.Namespace, collectionNameHack, o.Name)
}

/*


	eventsInformer := coreinformers.NewFilteredEventInformer(
		a.kubeClient, a.Namespace, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, nil)

	eventsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    a.addEvent,
		UpdateFunc: a.updateEvent,
	})

	logger.Debug("Starting eventsInformer...")
	stop := make(chan struct{})
	go eventsInformer.Run(stop)

	logger.Debug("waiting for caches to sync...")
	if ok := cache.WaitForCacheSync(stopCh, eventsInformer.HasSynced); !ok {
		return fmt.Errorf("failed to wait for events cache to sync")
	}
	logger.Debug("caches synced...")
	<-stopCh
	stop <- struct{}{}
	return nil

*/

type Adapter interface {
	Start(stopCh <-chan struct{}) error
}

type adapter struct {
	gvrs []schema.GroupVersionResource

	k8s       dynamic.Interface
	ce        cloudevents.Client
	source    string
	namespace string
	logger    *zap.SugaredLogger
}

func NewAdaptor(source string, namespace string, k8sClient dynamic.Interface, ceClient cloudevents.Client, logger *zap.SugaredLogger, gvr ...schema.GroupVersionResource) Adapter {
	a := &adapter{
		k8s:       k8sClient,
		ce:        ceClient,
		source:    source,
		namespace: namespace,
		gvrs:      gvr,
		logger:    logger,
	}
	return a
}

/*
TODO: No longer sending events for the controller of the updated object, a al:

	if controlled {
		informer.AddEventHandler(reconciler.Handler(impl.EnqueueControllerOf))
	} else {
		informer.AddEventHandler(reconciler.Handler(impl.Enqueue))
	}

*/

func (a *adapter) Start(stopCh <-chan struct{}) error {

	// TODO: the current duck informer has no namespace. fix that.

	// Local stop channel.
	stop := make(chan struct{})

	factory := duck.TypedInformerFactory{
		Client:       a.k8s,
		ResyncPeriod: time.Duration(10 * time.Hour),
		StopChannel:  stop,
		Type:         &duckv1alpha1.KResource{},
	}

	//dynamic.NamespaceableResourceInterface()

	//eventsInformer := coreinformers.NewFilteredEventInformer(
	//	a.kubeClient, a.Namespace, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, nil)

	//eventsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc:    a.addEvent,
	//	UpdateFunc: a.updateEvent,
	//})

	for _, gvr := range a.gvrs {
		informer, _, err := factory.GetNamespaced(gvr, a.namespace)
		if err != nil {
			return err
		}

		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    a.addEvent,
			UpdateFunc: a.updateEvent,
			DeleteFunc: a.deleteEvent,
		})
	}

	<-stopCh
	stop <- struct{}{}
	return nil
}

func (a *adapter) addEvent(obj interface{}) {
	object := obj.(*duckv1alpha1.KResource)

	if err := a.send(addEventType, object, &KubernetesEvent{
		Object: object,
	}); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
	}
}

func (a *adapter) updateEvent(oldObj, newObj interface{}) {
	object := newObj.(*duckv1alpha1.KResource)

	if err := a.send(updateEventType, object, &KubernetesEvent{
		NewObject: object,
		OldObject: oldObj.(*duckv1alpha1.KResource),
	}); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
	}
}

func (a *adapter) deleteEvent(obj interface{}) {
	object := obj.(*duckv1alpha1.KResource)

	if err := a.send(deleteEventType, object, &KubernetesEvent{
		Object: object,
	}); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
	}
}

func (a *adapter) send(eventType string, obj *duckv1alpha1.KResource, data *KubernetesEvent) error {
	subject := createSelfLink(corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
	})

	event := cloudevents.NewEvent()
	event.SetType(eventType)
	event.SetSource(a.source)
	event.SetSubject(subject)

	if err := event.SetData(data); err != nil {
		a.logger.Warn("failed to set event data for kubernetes event")
		return err
	}

	if _, err := a.ce.Send(context.TODO(), event); err != nil {
		return err
	}
	return nil
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	informer cache.SharedInformer,
	lister cache.GenericLister,
	eventsclient cloudevents.Client,
	controlled bool) *controller.Impl {

	r := &Reconciler{
		Base:         reconciler.NewBase(opt, controllerAgentName),
		lister:       lister,
		eventsClient: eventsclient,
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, r.Logger))

	r.Logger.Info("Setting up event handlers")

	if controlled {
		informer.AddEventHandler(reconciler.Handler(impl.EnqueueControllerOf))
	} else {
		informer.AddEventHandler(reconciler.Handler(impl.Enqueue))
	}
	return impl
}

// Reconciler reconciles an ApiServerSource object
type Reconciler struct {
	*reconciler.Base

	eventsClient cloudevents.Client
	lister       cache.GenericLister
}

// Reconcile sends a cloud event corresponding to the given key
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		r.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the resource with this namespace/name
	original, err := r.lister.ByNamespace(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		r.Logger.Error("resource key in work queue no longer exists", zap.Any("key", key))
		return nil
	} else if err != nil {
		return err
	}

	object := original.(*duckv1alpha1.AddressableType)

	eventType := updateEventType
	timestamp := object.GetCreationTimestamp()
	if object.GetDeletionTimestamp() != nil {
		eventType = deleteEventType
		timestamp = *object.GetDeletionTimestamp()
	}

	objectRef := corev1.ObjectReference{
		APIVersion: object.APIVersion,
		Kind:       object.Kind,
		Name:       object.GetName(),
		Namespace:  object.GetNamespace(),
	}

	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:     string(object.GetUID()),
			Type:   eventType,
			Source: *types.ParseURLRef(object.GetSelfLink()),
			Time:   &types.Timestamp{Time: timestamp.Time},
		}.AsV02(),
		Data: objectRef,
	}

	if _, err := r.eventsClient.Send(ctx, event); err != nil {
		r.Logger.Error("failed to send cloudevent (retrying)", err)

		return err
	}

	return nil
}
