/*
Copyright 2017 The Kubernetes Authors.

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

package stream

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"
	istiolisters "github.com/knative/eventing/pkg/client/listers/istio/v1alpha2"
	"github.com/knative/eventing/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	extensionslisters "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	streamscheme "github.com/knative/eventing/pkg/client/clientset/versioned/scheme"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	listers "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	elainformers "github.com/knative/serving/pkg/client/informers/externalversions"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	istiov1alpha2 "github.com/knative/eventing/pkg/apis/istio/v1alpha2"
)

const controllerAgentName = "stream-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Stream is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Stream fails
	// to sync due to a Service of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Service already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Stream"
	// MessageResourceSynced is the message used for an Event fired when a Stream
	// is synced successfully
	MessageResourceSynced = "Stream synced successfully"
)

// Controller is the controller implementation for Stream resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// streamclientset is a clientset for our own API group
	streamclientset clientset.Interface

	ingressesLister  extensionslisters.IngressLister
	ingressesSynced  cache.InformerSynced
	routerulesLister istiolisters.RouteRuleLister
	routerulesSynced cache.InformerSynced
	servicesLister   corelisters.ServiceLister
	servicesSynced   cache.InformerSynced
	streamsLister    listers.StreamLister
	streamsSynced    cache.InformerSynced

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

// NewController returns a new stream controller
func NewController(
	kubeclientset kubernetes.Interface,
	streamclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	streamInformerFactory informers.SharedInformerFactory,
	routeInformerFactory elainformers.SharedInformerFactory) controller.Interface {

	// obtain references to shared index informers for the Ingress, Service and Stream
	// types.
	ingressInformer := kubeInformerFactory.Extensions().V1beta1().Ingresses()
	routeruleInformer := streamInformerFactory.Config().V1alpha2().RouteRules()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	streamInformer := streamInformerFactory.Eventing().V1alpha1().Streams()

	// Create event broadcaster
	// Add stream-controller types to the default Kubernetes Scheme so Events can be
	// logged for stream-controller types.
	streamscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:    kubeclientset,
		streamclientset:  streamclientset,
		ingressesLister:  ingressInformer.Lister(),
		ingressesSynced:  ingressInformer.Informer().HasSynced,
		routerulesLister: routeruleInformer.Lister(),
		routerulesSynced: routeruleInformer.Informer().HasSynced,
		servicesLister:   serviceInformer.Lister(),
		servicesSynced:   serviceInformer.Informer().HasSynced,
		streamsLister:    streamInformer.Lister(),
		streamsSynced:    streamInformer.Informer().HasSynced,
		workqueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Streams"),
		recorder:         recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Stream resources change
	streamInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueStream,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueStream(new)
		},
	})
	// Set up an event handler for when Service resources change. This
	// handler will lookup the owner of the given Service, and if it is
	// owned by a Stream resource will enqueue that Stream resource for
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
	glog.Info("Starting Stream controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.ingressesSynced, c.servicesSynced, c.streamsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Stream resources
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
		// Stream resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Stream resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Stream resource with this namespace/name
	stream, err := c.streamsLister.Streams(namespace).Get(name)
	if err != nil {
		// The Stream resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("stream '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// Sync Service derived from the Stream
	service, err := c.syncStreamService(stream)
	if err != nil {
		return err
	}

	// Sync RouteRule derived from a brokered Stream
	brokeredStreamRouteRule, err := c.syncBrokeredStream(stream)
	if err != nil {
		return err
	}

	// Sync Ingress derived from the Stream
	ingress, err := c.syncStreamIngress(stream)
	if err != nil {
		return err
	}

	// Finally, we update the status block of the Stream resource to reflect the
	// current state of the world
	err = c.updateStreamStatus(stream, service, ingress, brokeredStreamRouteRule)
	if err != nil {
		return err
	}

	c.recorder.Event(stream, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) syncStreamService(stream *eventingv1alpha1.Stream) (*corev1.Service, error) {
	// Get the service with the specified service name
	serviceName := controller.StreamServiceName(stream.ObjectMeta.Name)
	service, err := c.servicesLister.Services(stream.Namespace).Get(serviceName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		service, err = c.kubeclientset.CoreV1().Services(stream.Namespace).Create(newService(stream))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}

	// If the Service is not controlled by this Stream resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(service, stream) {
		msg := fmt.Sprintf(MessageResourceExists, service.Name)
		c.recorder.Event(stream, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	return service, nil
}

func (c *Controller) syncBrokeredStream(stream *eventingv1alpha1.Stream) (*istiov1alpha2.RouteRule, error) {
	// Get the RouteRule with the specified Stream name
	routeruleName := controller.BrokeredStreamRouteRuleName(stream.ObjectMeta.Name)
	routerule, err := c.routerulesLister.RouteRules(stream.Namespace).Get(routeruleName)

	if stream.Spec.Broker == "" {
		// If the resource exists, delete it
		if routerule != nil {
			// Notify broker of stream removal
			err = deleteBrokeredStream(routerule.Labels["broker"], stream)
			if err != nil {
				return routerule, err
			}

			// Remove RouteRule
			err = c.streamclientset.ConfigV1alpha2().RouteRules(stream.Namespace).Delete(routeruleName, nil)
			if err != nil {
				return routerule, err
			}
		} else {
			// nothing to do
			return routerule, nil
		}
	}

	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		routerule, err = c.streamclientset.ConfigV1alpha2().RouteRules(stream.Namespace).Create(newBrokeredRouteRule(stream))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}

	// If the Service is not controlled by this Stream resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(routerule, stream) {
		msg := fmt.Sprintf(MessageResourceExists, routerule.Name)
		c.recorder.Event(stream, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	// Notify broker of stream
	err = provisionBrokeredStream(stream)
	if err != nil {
		return nil, err
	}

	return routerule, nil
}

func (c *Controller) syncStreamIngress(stream *eventingv1alpha1.Stream) (*extensionsv1beta1.Ingress, error) {
	// TODO make ingress optional

	// Get the ingress with the specified ingress name
	ingressName := controller.StreamIngressName(stream.ObjectMeta.Name)
	ingress, err := c.ingressesLister.Ingresses(stream.Namespace).Get(ingressName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		ingress, err = c.kubeclientset.ExtensionsV1beta1().Ingresses(stream.Namespace).Create(newIngress(stream))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}

	// If the Ingress is not controlled by this Stream resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(ingress, stream) {
		msg := fmt.Sprintf(MessageResourceExists, ingress.Name)
		c.recorder.Event(stream, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	return ingress, nil
}

func (c *Controller) updateStreamStatus(stream *eventingv1alpha1.Stream, service *corev1.Service, ingress *extensionsv1beta1.Ingress, brokeredStreamRouteRule *istiov1alpha2.RouteRule) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	streamCopy := stream.DeepCopy()
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Stream resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.streamclientset.EventingV1alpha1().Streams(stream.Namespace).Update(streamCopy)
	return err
}

// enqueueStream takes a Stream resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Stream.
func (c *Controller) enqueueStream(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Stream resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Stream resource to be processed. If the object does not
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
		// If this object is not owned by a Stream, we should not do anything more
		// with it.
		if ownerRef.Kind != "Stream" {
			return
		}

		stream, err := c.streamsLister.Streams(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of stream '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueStream(stream)
		return
	}
}

// newService creates a new Service for a Stream resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Stream resource that 'owns' it.
func newService(stream *eventingv1alpha1.Stream) *corev1.Service {
	labels := map[string]string{
		"stream": stream.Name,
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.StreamServiceName(stream.ObjectMeta.Name),
			Namespace: stream.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(stream, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Stream",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80},
			},
		},
	}
}

// newBrokeredRouteRule creates a new RouteRule for a brokered Stream resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Stream resource that 'owns' it.
func newBrokeredRouteRule(stream *eventingv1alpha1.Stream) *istiov1alpha2.RouteRule {
	labels := map[string]string{
		"broker": stream.Spec.Broker,
		"stream": stream.Name,
	}
	return &istiov1alpha2.RouteRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.BrokeredStreamRouteRuleName(stream.Name),
			Namespace: stream.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(stream, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Stream",
				}),
			},
		},
		Spec: istiov1alpha2.RouteRuleSpec{
			Destination: istiov1alpha2.IstioService{
				Name: controller.StreamServiceName(stream.Name),
			},
			Route: []istiov1alpha2.DestinationWeight{
				{
					Destination: istiov1alpha2.IstioService{
						Name: controller.BrokerServiceName(stream.Spec.Broker),
					},
					Weight: 100,
				},
			},
			Rewrite: istiov1alpha2.HTTPRewrite{
				Authority: fmt.Sprintf("%s.%s.streams.cluster.local", stream.Name, stream.Namespace),
			},
		},
	}
}

// newIngress creates a new Ingress for a Stream resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Stream resource that 'owns' it.
func newIngress(stream *eventingv1alpha1.Stream) *extensionsv1beta1.Ingress {
	labels := map[string]string{
		"stream": stream.Name,
	}
	annotations := map[string]string{
		"kubernetes.io/ingress.class": "istio",
	}
	return &extensionsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        controller.StreamIngressName(stream.ObjectMeta.Name),
			Namespace:   stream.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(stream, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Stream",
				}),
			},
		},
		Spec: extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{
				{
					// TODO make host name configurable
					Host: stream.ObjectMeta.Name,
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: []extensionsv1beta1.HTTPIngressPath{
								{
									Backend: extensionsv1beta1.IngressBackend{
										ServiceName: controller.StreamServiceName(stream.ObjectMeta.Name),
										ServicePort: intstr.FromString("http"),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func provisionBrokeredStream(stream *eventingv1alpha1.Stream) error {
	streamName := stream.Name
	namespace := stream.Namespace
	brokerName := stream.Spec.Broker
	brokerServiceName := controller.BrokerServiceName(brokerName)

	client := &http.Client{}
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local/streams/%s", brokerServiceName, namespace, streamName)
	fmt.Printf("Create stream at %s\n", url)
	request, err := http.NewRequest(http.MethodPut, url, strings.NewReader(""))
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Unable to provision brokered stream %s on %s", streamName, brokerName)
	}

	return nil
}

func deleteBrokeredStream(brokerName string, stream *eventingv1alpha1.Stream) error {
	streamName := stream.Name
	namespace := stream.Namespace
	brokerServiceName := controller.BrokerServiceName(brokerName)

	client := &http.Client{}
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local/streams/%s", brokerServiceName, namespace, streamName)
	fmt.Printf("Delete stream at %s\n", url)
	request, err := http.NewRequest(http.MethodDelete, url, strings.NewReader(""))
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Unable to delete brokered stream %s on %s", streamName, brokerName)
	}

	return nil
}
