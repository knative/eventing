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

package channel

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/controller"
	istiolisters "github.com/knative/pkg/client/listers/istio/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	channelscheme "github.com/knative/eventing/pkg/client/clientset/versioned/scheme"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	listers "github.com/knative/eventing/pkg/client/listers/channels/v1alpha1"
	"github.com/knative/eventing/pkg/system"
	sharedclientset "github.com/knative/pkg/client/clientset/versioned"
	sharedinformers "github.com/knative/pkg/client/informers/externalversions"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/controller/util"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
)

const controllerAgentName = "channel-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Channel is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Channel fails
	// to sync due to a Service of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Service already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Channel"
	// MessageResourceSynced is the message used for an Event fired when a Channel
	// is synced successfully
	MessageResourceSynced = "Channel synced successfully"
)

const (
	// ServiceSynced is used as part of the condition reason when the channel (k8s) service is successfully created.
	ServiceSynced = "ServiceSynced"
	// ServiceError is used as part of the condition reason when the channel (k8s) service creation failed.
	ServiceError = "ServiceError"
	// VirtualServiceSynced is used as part of the condition reason when the channel istio virtual service is successfully created.
	VirtualServiceSynced = "VirtualServiceSynced"
	// VirtualServiceError is used as part of the condition reason when the channel istio virtual service creation failed.
	VirtualServiceError = "VirtualServiceError"
)

const (
	PortNumber = 80
	PortName   = "http"
)

// Controller is the controller implementation for Channel resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// channelclientset is a clientset for our own API group
	channelclientset clientset.Interface
	// sharedclientset is a clientset for shared API groups
	sharedclientset sharedclientset.Interface

	virtualservicesLister istiolisters.VirtualServiceLister
	virtualservicesSynced cache.InformerSynced
	servicesLister        corelisters.ServiceLister
	servicesSynced        cache.InformerSynced
	channelsLister        listers.ChannelLister
	channelsSynced        cache.InformerSynced

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

// NewController returns a new channel controller
func NewController(
	kubeclientset kubernetes.Interface,
	channelclientset clientset.Interface,
	sharedclientset sharedclientset.Interface,
	restConfig *rest.Config,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	channelInformerFactory informers.SharedInformerFactory,
	sharedInformerFactory sharedinformers.SharedInformerFactory) controller.Interface {

	// obtain references to shared index informers for the Service and Channel types.
	virtualserviceInformer := sharedInformerFactory.Networking().V1alpha3().VirtualServices()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	channelInformer := channelInformerFactory.Channels().V1alpha1().Channels()

	// Create event broadcaster
	// Add channel-controller types to the default Kubernetes Scheme so Events can be
	// logged for channel-controller types.
	channelscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:         kubeclientset,
		channelclientset:      channelclientset,
		sharedclientset:       sharedclientset,
		virtualservicesLister: virtualserviceInformer.Lister(),
		virtualservicesSynced: virtualserviceInformer.Informer().HasSynced,
		servicesLister:        serviceInformer.Lister(),
		servicesSynced:        serviceInformer.Informer().HasSynced,
		channelsLister:        channelInformer.Lister(),
		channelsSynced:        channelInformer.Informer().HasSynced,
		workqueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Channels"),
		recorder:              recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Channel resources change
	channelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueChannel,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueChannel(new)
		},
	})
	// Set up an event handler for when Service resources change. This
	// handler will lookup the owner of the given Service, and if it is
	// owned by a Channel resource will enqueue that Channel resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Service resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newService := new.(*corev1.Service)
			oldService := old.(*corev1.Service)
			if newService.ResourceVersion == oldService.ResourceVersion {
				// Periodic resync will send update events for all known Services.
				// Two different versions of the same Service will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
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
	glog.Info("Starting Channel controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.servicesSynced, c.channelsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Channel resources
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
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Channel resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing channel '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced channel '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Channel resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Channel resource with this namespace/name
	channel, err := c.channelsLister.Channels(namespace).Get(name)
	if err != nil {
		// The Channel resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("channel '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	var service *corev1.Service
	var virtualService *istiov1alpha3.VirtualService
	var serviceErr, virtualServiceErr error

	// Sync Service derived from the Channel
	service, serviceErr = c.syncChannelService(channel)
	if err != nil {
		c.updateChannelStatus(channel, service, serviceErr, virtualService, virtualServiceErr)
		return err
	}

	// Sync VirtualService derived from a Channel
	virtualService, virtualServiceErr = c.syncChannelVirtualService(channel)
	if virtualServiceErr != nil {
		c.updateChannelStatus(channel, service, serviceErr, virtualService, virtualServiceErr)
		return err
	}

	// Finally, we update the status block of the Channel resource to reflect the
	// current state of the world
	err = c.updateChannelStatus(channel, service, serviceErr, virtualService, virtualServiceErr)
	if err != nil {
		return err
	}

	c.recorder.Event(channel, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) syncChannelService(channel *channelsv1alpha1.Channel) (*corev1.Service, error) {
	// Get the service with the specified service name
	serviceName := controller.ChannelServiceName(channel.ObjectMeta.Name)
	service, err := c.servicesLister.Services(channel.Namespace).Get(serviceName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		service, err = c.kubeclientset.CoreV1().Services(channel.Namespace).Create(newService(channel))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}

	// If the Service is not controlled by this Channel resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(service, channel) {
		msg := fmt.Sprintf(MessageResourceExists, service.Name)
		c.recorder.Event(channel, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	return service, nil
}

func (c *Controller) syncChannelVirtualService(channel *channelsv1alpha1.Channel) (*istiov1alpha3.VirtualService, error) {
	// Get the VirtualService with the specified Channel name
	virtualserviceName := controller.ChannelVirtualServiceName(channel.ObjectMeta.Name)
	virtualservice, err := c.virtualservicesLister.VirtualServices(channel.Namespace).Get(virtualserviceName)

	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		virtualservice, err = c.sharedclientset.NetworkingV1alpha3().VirtualServices(channel.Namespace).Create(newVirtualService(channel))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}

	// If the Service is not controlled by this Channel resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(virtualservice, channel) {
		msg := fmt.Sprintf(MessageResourceExists, virtualservice.Name)
		c.recorder.Event(channel, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	return virtualservice, nil
}

func (c *Controller) updateChannelStatus(channel *channelsv1alpha1.Channel,
	service *corev1.Service, serviceError error,
	virtualService *istiov1alpha3.VirtualService, virtualServiceError error) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	channelCopy := channel.DeepCopy()

	// Update ChannelStatus

	if service != nil {
		channelCopy.Status.Service = &corev1.LocalObjectReference{Name: service.Name}
		serviceCondition := util.NewChannelCondition(channelsv1alpha1.ChannelServiceable, corev1.ConditionTrue, ServiceSynced, "service successfully synced")
		util.SetChannelCondition(&channelCopy.Status, *serviceCondition)
	} else {
		channelCopy.Status.Service = nil
		serviceCondition := util.NewChannelCondition(channelsv1alpha1.ChannelServiceable, corev1.ConditionFalse, ServiceError, serviceError.Error())
		util.SetChannelCondition(&channelCopy.Status, *serviceCondition)
	}

	if virtualService != nil {
		channelCopy.Status.VirtualService = &corev1.LocalObjectReference{Name: virtualService.Name}
		serviceCondition := util.NewChannelCondition(channelsv1alpha1.ChannelRoutable, corev1.ConditionTrue, VirtualServiceSynced, "virtual service successfully synced")
		util.SetChannelCondition(&channelCopy.Status, *serviceCondition)
	} else {
		channelCopy.Status.VirtualService = nil
		serviceCondition := util.NewChannelCondition(channelsv1alpha1.ChannelRoutable, corev1.ConditionFalse, VirtualServiceError, virtualServiceError.Error())
		util.SetChannelCondition(&channelCopy.Status, *serviceCondition)
	}

	util.ConsolidateChannelCondition(&channelCopy.Status)

	channelCopy.Status.DomainInternal = controller.ServiceHostName(service.Name, service.Namespace)

	// Only update if status has changed
	if !equality.Semantic.DeepEqual(channel.Status, channelCopy.Status) {
		// If the CustomResourceSubresources feature gate is not enabled,
		// we must use Update instead of UpdateStatus to update the Status block of the Channel resource.
		// UpdateStatus will not allow changes to the Spec of the resource,
		// which is ideal for ensuring nothing other than resource status has been updated.
		_, err := c.channelclientset.ChannelsV1alpha1().Channels(channel.Namespace).Update(channelCopy)
		return err
	}
	return nil
}

// enqueueChannel takes a Channel resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Channel.
func (c *Controller) enqueueChannel(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Channel resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Channel resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Channel, we should not do anything more
		// with it.
		if ownerRef.Kind != "Channel" {
			return
		}

		channel, err := c.channelsLister.Channels(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of channel '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueChannel(channel)
		return
	}
}

// newService creates a new Service for a Channel resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Channel resource that 'owns' it.
func newService(channel *channelsv1alpha1.Channel) *corev1.Service {
	labels := map[string]string{
		"channel": channel.Name,
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ChannelServiceName(channel.ObjectMeta.Name),
			Namespace: channel.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(channel, schema.GroupVersionKind{
					Group:   channelsv1alpha1.SchemeGroupVersion.Group,
					Version: channelsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Channel",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: PortName, Port: PortNumber},
			},
		},
	}
}

// newVirtualService creates a new VirtualService for a Channel resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Channel resource that 'owns' it.
func newVirtualService(channel *channelsv1alpha1.Channel) *istiov1alpha3.VirtualService {
	labels := map[string]string{
		"channel": channel.Name,
	}
	var destinationHost string
	if len(channel.Spec.Bus) != 0 {
		labels["bus"] = channel.Spec.Bus
		destinationHost = controller.ServiceHostName(controller.BusDispatcherServiceName(channel.Spec.Bus), channel.Namespace)
	}
	if len(channel.Spec.ClusterBus) != 0 {
		labels["clusterBus"] = channel.Spec.ClusterBus
		destinationHost = controller.ServiceHostName(controller.ClusterBusDispatcherServiceName(channel.Spec.ClusterBus), system.Namespace)
	}
	return &istiov1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ChannelVirtualServiceName(channel.Name),
			Namespace: channel.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(channel, schema.GroupVersionKind{
					Group:   channelsv1alpha1.SchemeGroupVersion.Group,
					Version: channelsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Channel",
				}),
			},
		},
		Spec: istiov1alpha3.VirtualServiceSpec{
			Hosts: []string{
				controller.ServiceHostName(controller.ChannelServiceName(channel.Name), channel.Namespace),
				controller.ChannelHostName(channel.Name, channel.Namespace),
			},
			Http: []istiov1alpha3.HTTPRoute{{
				Rewrite: &istiov1alpha3.HTTPRewrite{
					Authority: controller.ChannelHostName(channel.Name, channel.Namespace),
				},
				Route: []istiov1alpha3.DestinationWeight{{
					Destination: istiov1alpha3.Destination{
						Host: destinationHost,
						Port: istiov1alpha3.PortSelector{
							Number: PortNumber,
						},
					}},
				}},
			},
		},
	}
}
