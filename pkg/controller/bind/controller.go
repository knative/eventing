/*
Copyright 2018 Google, Inc. All rights reserved.

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
	"time"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	elainformers "github.com/elafros/elafros/pkg/client/informers/externalversions"
	elalisters "github.com/elafros/elafros/pkg/client/listers/ela/v1alpha1"

	"github.com/elafros/eventing/pkg/controller"
	"github.com/elafros/eventing/pkg/triggers"

	v1alpha1 "github.com/elafros/eventing/pkg/apis/bind/v1alpha1"

	clientset "github.com/elafros/eventing/pkg/client/clientset/versioned"
	bindscheme "github.com/elafros/eventing/pkg/client/clientset/versioned/scheme"
	informers "github.com/elafros/eventing/pkg/client/informers/externalversions"
	listers "github.com/elafros/eventing/pkg/client/listers/bind/v1alpha1"
)

const controllerAgentName = "bind-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Bind is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Bind
	// is synced successfully
	MessageResourceSynced = "Bind synced successfully"
)

// Controller is the controller implementation for Bind resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// bindclientset is a clientset for our own API group
	bindclientset clientset.Interface

	bindsLister listers.BindLister
	bindsSynced cache.InformerSynced

	eventTypesLister listers.EventTypeLister
	eventTypesSynced cache.InformerSynced

	eventSourcesLister listers.EventSourceLister
	eventSourcesSynced cache.InformerSynced

	routesLister elalisters.RouteLister
	routesSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	triggers map[string]triggers.Trigger
}

// NewController returns a new bind controller
func NewController(
	kubeclientset kubernetes.Interface,
	bindclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	bindInformerFactory informers.SharedInformerFactory,
	routeInformerFactory elainformers.SharedInformerFactory) controller.Interface {

	// obtain a reference to a shared index informer for the Bind types.
	bindInformer := bindInformerFactory.Eventing().V1alpha1()

	// obtain a reference to a shared index informer for the Route type.
	routeInformer := routeInformerFactory.Elafros().V1alpha1().Routes()

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
		bindclientset:      bindclientset,
		bindsLister:        bindInformer.Binds().Lister(),
		bindsSynced:        bindInformer.Binds().Informer().HasSynced,
		routesLister:       routeInformer.Lister(),
		routesSynced:       routeInformer.Informer().HasSynced,
		eventSourcesLister: bindInformer.EventSources().Lister(),
		eventSourcesSynced: bindInformer.EventSources().Informer().HasSynced,
		eventTypesLister:   bindInformer.EventTypes().Lister(),
		eventTypesSynced:   bindInformer.EventTypes().Informer().HasSynced,
		workqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Binds"),
		recorder:           recorder,
	}

	controller.triggers = make(map[string]triggers.Trigger)
	controller.triggers["github"] = triggers.NewGithubTrigger()
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

	// Find the Route that they want.
	routeName := bind.Spec.Action.RouteName
	route, err := c.routesLister.Routes(namespace).Get(routeName)
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("Route %q in namespace %q does not exist", routeName, namespace))
		}
		return err
	}

	domainSuffix, err := getDomainSuffixFromElaConfig(c.kubeclientset)
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("Can't find ela ConfigMap"))
		}
		return err
	}

	functionDNS := fmt.Sprintf("%s.%s.%s", route.Name, route.Namespace, domainSuffix)
	glog.Infof("Found route DNS as '%q'", functionDNS)

	es, err := c.eventSourcesLister.EventSources(namespace).Get(bind.Spec.Source.EventSource)
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("EventSource %q in namespace %q does not exist", bind.Spec.Source.EventSource, namespace))
		}
		return err
	}

	et, err := c.eventTypesLister.EventTypes(namespace).Get(bind.Spec.Source.EventType)
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("EventType %q in namespace %q does not exist", bind.Spec.Source.EventSource, namespace))
		}
		return err
	}

	params, err := getParameters(c.kubeclientset, namespace, bind.Spec.Parameters.Raw, bind.Spec.ParametersFrom)
	if err != nil {
		glog.Warningf("Failed to process parameters: %s", err)
		return err
	}

	if bind.Status.Conditions != nil {
		glog.Infof("Already has status, skipping")
		return nil
	}

	glog.Infof("Creating a subscription to %q : %q with Parameters %+v", es.Name, et.Name, params)
	if val, ok := c.triggers[es.Name]; ok {
		r, err := val.Bind(params, functionDNS)
		if err != nil {
			glog.Warningf("BIND failed: %s", err)
			msg := fmt.Sprintf("Bind failed with : %s", r)
			bind.Status.SetCondition(&v1alpha1.BindCondition{
				Type:    v1alpha1.BindFailed,
				Status:  corev1.ConditionTrue,
				Reason:  "BindFailed",
				Message: msg,
			})
		} else {
			bind.Status.SetCondition(&v1alpha1.BindCondition{
				Type:    v1alpha1.BindComplete,
				Status:  corev1.ConditionTrue,
				Reason:  fmt.Sprintf("BindSuccess: Hook: %s", r["ID"].(string)),
				Message: "Bind successful",
			})
		}
	}
	_, err = c.updateStatus(bind)
	if err != nil {
		glog.Warningf("Failed to update status: %s", err)
		return err
	}

	c.recorder.Event(bind, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateStatus(u *v1alpha1.Bind) (*v1alpha1.Bind, error) {
	bindClient := c.bindclientset.EventingV1alpha1().Binds(u.Namespace)
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

func getParameters(kubeClient kubernetes.Interface, namespace string, parameters []byte, parametersFrom []v1alpha1.ParametersFromSource) (map[string]interface{}, error) {
	r := make(map[string]interface{})
	if parameters != nil && len(parameters) > 0 {
		p := make(map[string]interface{})
		if err := yaml.Unmarshal(parameters, &p); err != nil {
			return nil, err
		}
		for k, v := range p {
			r[k] = v
		}
	}
	if parametersFrom != nil {
		glog.Infof("Fetching from source %+v", parametersFrom)
		for _, p := range parametersFrom {
			pfs, err := fetchParametersFromSource(kubeClient, namespace, &p)
			if err != nil {
				return nil, err
			}
			for k, v := range pfs {
				r[k] = v
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

func getDomainSuffixFromElaConfig(cl kubernetes.Interface) (string, error) {
	const name = "ela-config"
	const namespace = "ela-system"
	c, err := cl.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	domainSuffix, ok := c.Data["domainSuffix"]
	if !ok {
		return "", fmt.Errorf("cannot find domainSuffix in %v: ConfigMap.Data is %#v", name, c.Data)
	}
	return domainSuffix, nil
}
