/*
Copyright 2017 The Knative Authors

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

package clusterbus

import (
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/controller"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
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
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	channelscheme "github.com/knative/eventing/pkg/client/clientset/versioned/scheme"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	listers "github.com/knative/eventing/pkg/client/listers/channels/v1alpha1"
	"github.com/knative/eventing/pkg/system"
	sharedclientset "github.com/knative/pkg/client/clientset/versioned"
	sharedinformers "github.com/knative/pkg/client/informers/externalversions"
)

const (
	controllerAgentName = "clusterbus-controller"
	serviceAccountName  = "bus-operator"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a ClusterBus is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a ClusterBus fails
	// to sync due to a Service of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Service already existing
	MessageResourceExists = "Resource %q already exists and is not managed by ClusterBus"
	// MessageResourceSynced is the message used for an Event fired when a ClusterBus
	// is synced successfully
	MessageResourceSynced = "ClusterBus synced successfully"
)

// Controller is the controller implementation for ClusterBus resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// clusterbusclientset is a clientset for our own API group
	clusterbusclientset clientset.Interface

	deploymentsLister  appslisters.DeploymentLister
	deploymentsSynced  cache.InformerSynced
	servicesLister     corelisters.ServiceLister
	servicesSynced     cache.InformerSynced
	clusterBusesLister listers.ClusterBusLister
	clusterBusesSynced cache.InformerSynced

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

// NewController returns a new clusterbus controller
func NewController(
	kubeclientset kubernetes.Interface,
	clusterbusclientset clientset.Interface,
	sharedclientset sharedclientset.Interface,
	restConfig *rest.Config,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	clusterBusInformerFactory informers.SharedInformerFactory,
	sharedInformerFactory sharedinformers.SharedInformerFactory,
) controller.Interface {

	// obtain references to shared index informers for the ClusterBus, Deployment and Service
	// types.
	clusterBusInformer := clusterBusInformerFactory.Channels().V1alpha1().ClusterBuses()
	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()
	serviceInformer := kubeInformerFactory.Core().V1().Services()

	// Create event broadcaster
	// Add clusterbus-controller types to the default Kubernetes Scheme so Events can be
	// logged for clusterbus-controller types.
	channelscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:       kubeclientset,
		clusterbusclientset: clusterbusclientset,
		deploymentsLister:   deploymentInformer.Lister(),
		deploymentsSynced:   deploymentInformer.Informer().HasSynced,
		servicesLister:      serviceInformer.Lister(),
		servicesSynced:      serviceInformer.Informer().HasSynced,
		clusterBusesLister:  clusterBusInformer.Lister(),
		clusterBusesSynced:  clusterBusInformer.Informer().HasSynced,
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ClusterBuses"),
		recorder:            recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when ClusterBus resources change
	clusterBusInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueClusterBus,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueClusterBus(new)
		},
	})
	// Set up an event handler for when Service resources change. This
	// handler will lookup the owner of the given Service, and if it is
	// owned by a ClusterBus resource will enqueue that ClusterBus resource for
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
	glog.Info("Starting ClusterBus controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.servicesSynced, c.clusterBusesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process ClusterBus resources
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
		// ClusterBus resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing clusterbus '%s': %v", key, err)
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced clusterbus '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the ClusterBus resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the ClusterBus resource with this name
	clusterBus, err := c.clusterBusesLister.Get(name)
	if err != nil {
		// The ClusterBus resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("clusterbus '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// Sync Service derived from the ClusterBus
	dispatcherService, err := c.syncClusterBusDispatcherService(clusterBus)
	if err != nil {
		return err
	}

	// Sync Deployment derived from the ClusterBus
	dispatcherDeployment, err := c.syncClusterBusDispatcherDeployment(clusterBus)
	if err != nil {
		return err
	}

	// Sync Deployment derived from the ClusterBus
	provisionerDeployment, err := c.syncClusterBusProvisionerDeployment(clusterBus)
	if err != nil {
		return err
	}

	// Finally, we update the status block of the ClusterBus resource to reflect the
	// current state of the world
	err = c.updateClusterBusStatus(clusterBus, dispatcherService, dispatcherDeployment, provisionerDeployment)
	if err != nil {
		return err
	}

	c.recorder.Event(clusterBus, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) syncClusterBusDispatcherService(clusterBus *channelsv1alpha1.ClusterBus) (*corev1.Service, error) {
	// Get the service with the specified service name
	serviceName := controller.ClusterBusDispatcherServiceName(clusterBus.ObjectMeta.Name)
	service, err := c.servicesLister.Services(system.Namespace).Get(serviceName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		service, err = c.kubeclientset.CoreV1().Services(system.Namespace).Create(newDispatcherService(clusterBus))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}

	// If the Service is not controlled by this ClusterBus resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(service, clusterBus) {
		msg := fmt.Sprintf(MessageResourceExists, service.Name)
		c.recorder.Event(clusterBus, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	return service, nil
}

func (c *Controller) syncClusterBusDispatcherDeployment(clusterBus *channelsv1alpha1.ClusterBus) (*appsv1.Deployment, error) {
	// Get the deployment with the specified deployment name
	deploymentName := controller.ClusterBusDispatcherDeploymentName(clusterBus.ObjectMeta.Name)
	deployment, err := c.deploymentsLister.Deployments(system.Namespace).Get(deploymentName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(system.Namespace).Create(newDispatcherDeployment(clusterBus))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}

	// If the Deployment is not controlled by this ClusterBus resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(deployment, clusterBus) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(clusterBus, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	// If the Deployment does not match the ClusterBus's proposed Deployment we should update
	// the Deployment resource.
	proposedDeployment := newDispatcherDeployment(clusterBus)
	if !reflect.DeepEqual(proposedDeployment.Spec, deployment.Spec) {
		glog.V(4).Infof("ClusterBus %s dispatcher spec updated", clusterBus.Name)
		deployment, err = c.kubeclientset.AppsV1().Deployments(system.Namespace).Update(proposedDeployment)

		if err != nil {
			return nil, err
		}
	}

	return deployment, nil
}

func (c *Controller) syncClusterBusProvisionerDeployment(clusterBus *channelsv1alpha1.ClusterBus) (*appsv1.Deployment, error) {
	provisioner := clusterBus.Spec.Provisioner

	// Get the deployment with the specified deployment name
	deploymentName := controller.ClusterBusProvisionerDeploymentName(clusterBus.ObjectMeta.Name)
	deployment, err := c.deploymentsLister.Deployments(system.Namespace).Get(deploymentName)

	// If the resource shouldn't exists
	if provisioner == nil {
		// If the resource exists, we'll delete it
		if deployment != nil {
			err = c.kubeclientset.AppsV1().Deployments(system.Namespace).Delete(deploymentName, nil)
		}
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(system.Namespace).Create(newProvisionerDeployment(clusterBus))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}

	// If the Deployment is not controlled by this ClusterBus resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(deployment, clusterBus) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(clusterBus, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	// If the Deployment does not match the ClusterBus's proposed Deployment we should update
	// the Deployment resource.
	proposedDeployment := newProvisionerDeployment(clusterBus)
	if !reflect.DeepEqual(proposedDeployment.Spec, deployment.Spec) {
		glog.V(4).Infof("ClusterBus %s provisioner spec updated", clusterBus.Name)
		deployment, err = c.kubeclientset.AppsV1().Deployments(system.Namespace).Update(proposedDeployment)

		if err != nil {
			return nil, err
		}
	}

	return deployment, nil
}

func (c *Controller) updateClusterBusStatus(
	clusterBus *channelsv1alpha1.ClusterBus,
	dispatcherService *corev1.Service,
	dispatcherDeployment *appsv1.Deployment,
	provisionerDeployment *appsv1.Deployment,
) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	clusterBusCopy := clusterBus.DeepCopy()
	// Only update if status has changed
	if !equality.Semantic.DeepEqual(clusterBus.Status, clusterBusCopy.Status) {
		// If the CustomResourceSubresources feature gate is not enabled,
		// we must use Update instead of UpdateStatus to update the Status block of the ClusterBus resource.
		// UpdateStatus will not allow changes to the Spec of the resource,
		// which is ideal for ensuring nothing other than resource status has been updated.
		_, err := c.clusterbusclientset.ChannelsV1alpha1().ClusterBuses().Update(clusterBusCopy)
		return err
	}
	return nil
}

// enqueueClusterBus takes a ClusterBus resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than ClusterBus.
func (c *Controller) enqueueClusterBus(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the ClusterBus resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that ClusterBus resource to be processed. If the object does not
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
		// If this object is not owned by a ClusterBus, we should not do anything more
		// with it.
		if ownerRef.Kind != "ClusterBus" {
			return
		}

		clusterBus, err := c.clusterBusesLister.Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of clusterbus '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueClusterBus(clusterBus)
		return
	}
}

// newDispatcherService creates a new Service for a ClusterBus resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ClusterBus resource that 'owns' it.
func newDispatcherService(clusterBus *channelsv1alpha1.ClusterBus) *corev1.Service {
	labels := dispatcherLabels(clusterBus.Name)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ClusterBusDispatcherServiceName(clusterBus.ObjectMeta.Name),
			Namespace: system.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(clusterBus, schema.GroupVersionKind{
					Group:   channelsv1alpha1.SchemeGroupVersion.Group,
					Version: channelsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "ClusterBus",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
}

// newDispatcherDeployment creates a new Deployment for a ClusterBus resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ClusterBus resource that 'owns' it.
func newDispatcherDeployment(clusterBus *channelsv1alpha1.ClusterBus) *appsv1.Deployment {
	labels := dispatcherLabels(clusterBus.Name)
	one := int32(1)
	container := clusterBus.Spec.Dispatcher.DeepCopy()
	container.Env = append(container.Env,
		corev1.EnvVar{
			Name:  "PORT",
			Value: "8080",
		},
		corev1.EnvVar{
			Name:  "BUS_NAME",
			Value: clusterBus.Name,
		},
	)
	volumes := []corev1.Volume{}
	if clusterBus.Spec.Volumes != nil {
		volumes = *clusterBus.Spec.Volumes
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ClusterBusDispatcherDeploymentName(clusterBus.ObjectMeta.Name),
			Namespace: system.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(clusterBus, schema.GroupVersionKind{
					Group:   channelsv1alpha1.SchemeGroupVersion.Group,
					Version: channelsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "ClusterBus",
				}),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					Containers: []corev1.Container{
						*container,
					},
					Volumes: volumes,
				},
			},
		},
	}
}

// newProvisionerDeployment creates a new Deployment for a ClusterBus resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ClusterBus resource that 'owns' it.
func newProvisionerDeployment(clusterBus *channelsv1alpha1.ClusterBus) *appsv1.Deployment {
	labels := provisionerLabels(clusterBus.Name)
	one := int32(1)
	container := clusterBus.Spec.Provisioner.DeepCopy()
	container.Env = append(container.Env,
		corev1.EnvVar{
			Name:  "BUS_NAME",
			Value: clusterBus.Name,
		},
	)
	volumes := []corev1.Volume{}
	if clusterBus.Spec.Volumes != nil {
		volumes = *clusterBus.Spec.Volumes
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ClusterBusProvisionerDeploymentName(clusterBus.ObjectMeta.Name),
			Namespace: system.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(clusterBus, schema.GroupVersionKind{
					Group:   channelsv1alpha1.SchemeGroupVersion.Group,
					Version: channelsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "ClusterBus",
				}),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					Containers: []corev1.Container{
						*container,
					},
					Volumes: volumes,
				},
			},
		},
	}
}

func dispatcherLabels(busName string) map[string]string {
	return map[string]string{
		"clusterBus": busName,
		"role":       "dispatcher",
	}
}

func provisionerLabels(busName string) map[string]string {
	return map[string]string{
		"clusterBus": busName,
		"role":       "provisioner",
	}
}
