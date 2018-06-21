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

package bind

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimetypes "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	servinginformers "github.com/knative/serving/pkg/client/informers/externalversions"
	servinglisters "github.com/knative/serving/pkg/client/listers/serving/v1alpha1"

	"github.com/knative/eventing/pkg/controller"
	"github.com/knative/eventing/pkg/sources"

	v1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"

	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	bindscheme "github.com/knative/eventing/pkg/client/clientset/versioned/scheme"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	channelListers "github.com/knative/eventing/pkg/client/listers/channels/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/feeds/v1alpha1"
)

const controllerAgentName = "bind-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Bind is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Bind
	// is synced successfully
	MessageResourceSynced = "Bind synced successfully"
)

var (
	bindControllerKind      = v1alpha1.SchemeGroupVersion.WithKind("Bind")
	eventTypeControllerKind = v1alpha1.SchemeGroupVersion.WithKind("EventType")
)

// Controller is the controller implementation for Bind resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	// feedsclientset is a clientset for our own API group
	feedsclientset clientset.Interface

	bindsLister listers.BindLister
	bindsSynced cache.InformerSynced

	eventTypesLister listers.EventTypeLister
	eventTypesSynced cache.InformerSynced

	eventSourcesLister listers.EventSourceLister
	eventSourcesSynced cache.InformerSynced

	routesLister servinglisters.RouteLister
	routesSynced cache.InformerSynced

	channelsLister channelListers.ChannelLister
	channelsSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new bind controller
func NewController(
	kubeclientset kubernetes.Interface,
	feedsclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	feedsInformerFactory informers.SharedInformerFactory,
	routeInformerFactory servinginformers.SharedInformerFactory) controller.Interface {

	// obtain a reference to a shared index informer for the Bind types.
	bindInformer := feedsInformerFactory.Feeds().V1alpha1()

	// obtain a reference to a shared index informer for the Route type.
	routeInformer := routeInformerFactory.Serving().V1alpha1().Routes()

	channelInformer := feedsInformerFactory.Channels().V1alpha1().Channels()

	// Create event broadcaster
	// Add bind-controller types to the default Kubernetes Scheme so Events can be
	// logged for bind-controller types.
	bindscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:      kubeclientset,
		feedsclientset:     feedsclientset,
		bindsLister:        bindInformer.Binds().Lister(),
		bindsSynced:        bindInformer.Binds().Informer().HasSynced,
		routesLister:       routeInformer.Lister(),
		routesSynced:       routeInformer.Informer().HasSynced,
		eventSourcesLister: bindInformer.EventSources().Lister(),
		eventSourcesSynced: bindInformer.EventSources().Informer().HasSynced,
		eventTypesLister:   bindInformer.EventTypes().Lister(),
		eventTypesSynced:   bindInformer.EventTypes().Informer().HasSynced,
		channelsLister:     channelInformer.Lister(),
		channelsSynced:     channelInformer.Informer().HasSynced,
		workqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Binds"),
		recorder:           recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Bind resources change
	bindInformer.Binds().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueBind,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueBind(new)
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Bind controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for Bind informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.bindsSynced); !ok {
		return fmt.Errorf("failed to wait for Bind caches to sync")
	}

	glog.Info("Waiting for EventSources informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.eventSourcesSynced); !ok {
		return fmt.Errorf("failed to wait for EventSources caches to sync")
	}

	glog.Info("Waiting for EventTypes informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.eventTypesSynced); !ok {
		return fmt.Errorf("failed to wait for EventTypes caches to sync")
	}

	glog.Info("Waiting for route informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.routesSynced); !ok {
		return fmt.Errorf("failed to wait for Route caches to sync")
	}

	glog.Info("Waiting for channel informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.channelsSynced); !ok {
		return fmt.Errorf("failed to wait for Channel caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Bind resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	if err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		key, ok := obj.(string)
		if !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Bind resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj); err != nil {
		runtime.HandleError(err)
	}

	return true
}

// enqueueBind takes a Bind resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Bind.
func (c *Controller) enqueueBind(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Bind resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Bind resource with this namespace/name
	bind, err := c.bindsLister.Binds(namespace).Get(name)
	if err != nil {
		// The Bind resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("bind '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}
	// Don't mutate the informer's copy of our object.
	bind = bind.DeepCopy()

	// See if the binding has been deleted
	accessor, err := meta.Accessor(bind)
	if err != nil {
		log.Fatalf("Failed to get metadata: %s", err)
	}
	deletionTimestamp := accessor.GetDeletionTimestamp()
	glog.Infof("DeletionTimestamp: %v", deletionTimestamp)

	functionDNS, err := c.resolveActionTarget(bind.Namespace, bind.Spec.Action)

	// Only return an error on not found if we're not deleting so that we can delete
	// the binding even if the route or channel has already been deleted.
	if err != nil && deletionTimestamp == nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("cannot resolve target for %v in namespace %q", bind.Spec.Action, namespace))
		}
		return err
	}

	es, err := c.eventSourcesLister.EventSources(namespace).Get(bind.Spec.Trigger.Service)
	if err != nil && deletionTimestamp == nil {
		if errors.IsNotFound(err) {
			if deletionTimestamp != nil {
				// If the Event Source can not be found, we will remove our finalizer
				// because without it, we can't unbind and hence this binding will never
				// be deleted.
				// https://github.com/knative/eventing/issues/94
				newFinalizers, err := RemoveFinalizer(bind, controllerAgentName)
				if err != nil {
					glog.Warningf("Failed to remove finalizer: %s", err)
					return err
				}
				bind.ObjectMeta.Finalizers = newFinalizers
				_, err = c.updateFinalizers(bind)
				if err != nil {
					glog.Warningf("Failed to update finalizers: %s", err)
					return err
				}
				return nil
			}
			runtime.HandleError(fmt.Errorf("EventSource %q in namespace %q does not exist", bind.Spec.Trigger.Service, namespace))
		}
		return err
	}

	et, err := c.eventTypesLister.EventTypes(namespace).Get(bind.Spec.Trigger.EventType)
	if err != nil && deletionTimestamp == nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("EventType %q in namespace %q does not exist", bind.Spec.Trigger.Service, namespace))
		}
		return err
	}

	// If the EventSource has been deleted from underneath us, just remove our finalizer. We tried...
	if es == nil && deletionTimestamp != nil {
		glog.Warningf("Could not find a Bind container, removing finalizer")
		newFinalizers, err := RemoveFinalizer(bind, controllerAgentName)
		if err != nil {
			glog.Warningf("Failed to remove finalizer: %s", err)
			return err
		}
		bind.ObjectMeta.Finalizers = newFinalizers
		_, err = c.updateFinalizers(bind)
		if err != nil {
			glog.Warningf("Failed to update finalizers: %s", err)
			return err
		}
		return nil
	}

	// If there are conditions or a context do nothing.
	if (bind.Status.Conditions != nil || bind.Status.BindContext != nil) && deletionTimestamp == nil {
		glog.Infof("Binding \"%s/%s\" already has status, skipping", bind.Namespace, bind.Name)
		return nil
	}

	// Set the OwnerReference to EventType to make sure that if it's deleted, the binding
	// will also get deleted and not left orphaned. However, this does not work yet. Regardless
	// we should set the owner reference to indicate a dependency.
	// https://github.com/knative/eventing/issues/94
	bind.ObjectMeta.OwnerReferences = append(bind.ObjectMeta.OwnerReferences, *newEventTypeNonControllerRef(et))
	bindClient := c.feedsclientset.FeedsV1alpha1().Binds(bind.Namespace)
	updatedBind, err := bindClient.Update(bind)
	if err != nil {
		glog.Warningf("Failed to update OwnerReferences on bind '%s/%s' : %v", bind.Namespace, bind.Name, err)
		return err
	}
	bind = updatedBind

	trigger, err := resolveTrigger(c.kubeclientset, namespace, bind.Spec.Trigger)
	if err != nil {
		glog.Warningf("Failed to process parameters: %s", err)
		return err
	}

	// Don't mutate the informer's copy of our object.
	newES := es.DeepCopy()

	// check if the user specified a ServiceAccount to use and if so, use it.
	serviceAccountName := "default"
	if len(bind.Spec.ServiceAccountName) != 0 {
		serviceAccountName = bind.Spec.ServiceAccountName
	}
	binder := sources.NewContainerEventSource(bind, c.kubeclientset, &newES.Spec, bind.Namespace, serviceAccountName)
	if deletionTimestamp == nil {
		glog.Infof("Creating a subscription to %q : %q for resource %q", es.Name, et.Name, trigger.Resource)
		bindContext, err := binder.Bind(trigger, functionDNS)

		if err != nil {
			glog.Warningf("BIND failed: %s", err)
			msg := fmt.Sprintf("Bind failed with : %s", err)
			bind.Status.SetCondition(&v1alpha1.BindCondition{
				Type:    v1alpha1.BindFailed,
				Status:  corev1.ConditionTrue,
				Reason:  "BindFailed",
				Message: msg,
			})
		} else {
			glog.Infof("Got context back as: %+v", bindContext)
			marshalledBindContext, err := json.Marshal(&bindContext.Context)
			if err != nil {
				glog.Warningf("Couldn't marshal bind context: %+v : %s", bindContext, err)
			} else {
				glog.Infof("Marshaled context to: %+v", marshalledBindContext)
				bind.Status.BindContext = &runtimetypes.RawExtension{
					Raw: make([]byte, len(marshalledBindContext)),
				}
				bind.Status.BindContext.Raw = marshalledBindContext
			}

			// Set the finalizer since the bind succeeded, we need to clean up...
			// TODO: we should do this in the webhook instead...
			bind.Finalizers = append(bind.ObjectMeta.Finalizers, controllerAgentName)
			_, err = c.updateFinalizers(bind)
			if err != nil {
				glog.Warningf("Failed to update finalizers: %s", err)
				return err
			}

			bind.Status.SetCondition(&v1alpha1.BindCondition{
				Type:    v1alpha1.BindComplete,
				Status:  corev1.ConditionTrue,
				Reason:  "BindSuccess",
				Message: "Bind successful",
			})
		}
		_, err = c.updateStatus(bind)
		if err != nil {
			glog.Warningf("Failed to update status: %s", err)
			return err
		}
	} else {
		glog.Infof("Deleting a subscription to %q : %q with Trigger %+v", es.Name, et.Name, trigger)
		bindContext := sources.BindContext{
			Context: make(map[string]interface{}),
		}
		if bind.Status.BindContext != nil && bind.Status.BindContext.Raw != nil && len(bind.Status.BindContext.Raw) > 0 {
			if err := json.Unmarshal(bind.Status.BindContext.Raw, &bindContext.Context); err != nil {
				glog.Warningf("Couldn't unmarshal BindContext: %v", err)
				// TODO set the condition properly here
				return err
			}
		}
		err := binder.Unbind(trigger, bindContext)
		if err != nil {
			glog.Warningf("Couldn't unbind: %v", err)
			// TODO set the condition properly here
			return err
		}
		newFinalizers, err := RemoveFinalizer(bind, controllerAgentName)
		if err != nil {
			glog.Warningf("Failed to remove finalizer: %s", err)
			return err
		}
		bind.ObjectMeta.Finalizers = newFinalizers
		_, err = c.updateFinalizers(bind)
		if err != nil {
			glog.Warningf("Failed to update finalizers: %s", err)
			return err
		}
	}

	c.recorder.Event(bind, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateFinalizers(u *v1alpha1.Bind) (*v1alpha1.Bind, error) {
	bindClient := c.feedsclientset.FeedsV1alpha1().Binds(u.Namespace)
	newu, err := bindClient.Get(u.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	newu.ObjectMeta.Finalizers = u.ObjectMeta.Finalizers
	return bindClient.Update(newu)
}

func (c *Controller) updateStatus(u *v1alpha1.Bind) (*v1alpha1.Bind, error) {
	bindClient := c.feedsclientset.FeedsV1alpha1().Binds(u.Namespace)
	newu, err := bindClient.Get(u.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	newu.Status = u.Status

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Bind resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	return bindClient.Update(newu)
}

func resolveTrigger(kubeClient kubernetes.Interface, namespace string, trigger v1alpha1.EventTrigger) (sources.EventTrigger, error) {
	r := sources.EventTrigger{
		Resource:   trigger.Resource,
		EventType:  trigger.EventType,
		Parameters: make(map[string]interface{}),
	}
	if trigger.Parameters != nil && trigger.Parameters.Raw != nil && len(trigger.Parameters.Raw) > 0 {
		p := make(map[string]interface{})
		if err := yaml.Unmarshal(trigger.Parameters.Raw, &p); err != nil {
			return r, err
		}
		for k, v := range p {
			r.Parameters[k] = v
		}
	}
	if trigger.ParametersFrom != nil {
		glog.Infof("Fetching from source %+v", trigger.ParametersFrom)
		for _, p := range trigger.ParametersFrom {
			pfs, err := fetchParametersFromSource(kubeClient, namespace, &p)
			if err != nil {
				return r, err
			}
			for k, v := range pfs {
				r.Parameters[k] = v
			}
		}
	}
	return r, nil
}

func fetchParametersFromSource(kubeClient kubernetes.Interface, namespace string, parametersFrom *v1alpha1.ParametersFromSource) (map[string]interface{}, error) {
	var params map[string]interface{}
	if parametersFrom.SecretKeyRef != nil {
		glog.Infof("Fetching secret %+v", parametersFrom.SecretKeyRef)
		data, err := fetchSecretKeyValue(kubeClient, namespace, parametersFrom.SecretKeyRef)
		if err != nil {
			return nil, err
		}

		p, err := unmarshalJSON(data)
		if err != nil {
			return nil, err
		}
		params = p

	}
	return params, nil
}

func fetchSecretKeyValue(kubeClient kubernetes.Interface, namespace string, secretKeyRef *v1alpha1.SecretKeyReference) ([]byte, error) {
	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(secretKeyRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return secret.Data[secretKeyRef.Key], nil
}

func unmarshalJSON(in []byte) (map[string]interface{}, error) {
	parameters := make(map[string]interface{})
	if err := json.Unmarshal(in, &parameters); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parameters as JSON object: %v", err)
	}
	return parameters, nil
}

// AddFinalizer adds value to the list of finalizers on obj
func AddFinalizer(obj runtimetypes.Object, value string) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	finalizers := sets.NewString(accessor.GetFinalizers()...)
	finalizers.Insert(value)
	accessor.SetFinalizers(finalizers.List())
	return nil
}

// RemoveFinalizer removes the given value from the list of finalizers in obj, then returns a new list
// of finalizers after value has been removed.
func RemoveFinalizer(obj runtimetypes.Object, value string) ([]string, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	finalizers := sets.NewString(accessor.GetFinalizers()...)
	finalizers.Delete(value)
	newFinalizers := finalizers.List()
	accessor.SetFinalizers(newFinalizers)
	return newFinalizers, nil
}

func newEventTypeNonControllerRef(et *v1alpha1.EventType) *metav1.OwnerReference {
	blockOwnerDeletion := true
	isController := false
	revRef := metav1.NewControllerRef(et, eventTypeControllerKind)
	revRef.BlockOwnerDeletion = &blockOwnerDeletion
	revRef.Controller = &isController
	return revRef
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Bind resource
// with the current status of the resource.
func (c *Controller) resolveActionTarget(namespace string, action v1alpha1.BindAction) (string, error) {
	if len(action.RouteName) > 0 {
		return c.resolveRouteDNS(namespace, action.RouteName)
	}
	if len(action.ChannelName) > 0 {
		return c.resolveChannelDNS(namespace, action.ChannelName)
	}
	// This should never happen, but because we don't have webhook validation yet, check
	// and complain.
	return "", fmt.Errorf("action is missing both RouteName and ChannelName")
}

func (c *Controller) resolveRouteDNS(namespace string, routeName string) (string, error) {
	route, err := c.routesLister.Routes(namespace).Get(routeName)
	if err != nil {
		return "", err
	}
	if len(route.Status.Domain) == 0 {
		return "", fmt.Errorf("route '%s/%s' is missing a domain", namespace, routeName)
	}
	return route.Status.Domain, nil
}

func (c *Controller) resolveChannelDNS(namespace string, channelName string) (string, error) {
	channel, err := c.channelsLister.Channels(namespace).Get(channelName)
	if err != nil {
		return "", err
	}
	// TODO: The actual dns name should come from something in the status, or ?? But right
	// now it is hard coded to be <channelname>-channel
	// So we just check that the channel actually exists and tack on the -channel
	return fmt.Sprintf("%s-channel", channel.Name), nil
}
