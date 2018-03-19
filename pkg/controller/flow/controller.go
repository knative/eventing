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

package flow

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
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

	v1beta2 "github.com/elafros/eventing/pkg/apis/eventing/v1beta2"

	clientset "github.com/elafros/eventing/pkg/client/clientset/versioned"
	flowscheme "github.com/elafros/eventing/pkg/client/clientset/versioned/scheme"
	informers "github.com/elafros/eventing/pkg/client/informers/externalversions"
)

const controllerAgentName = "flow-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Flow is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Flow
	// is synced successfully
	MessageResourceSynced = "Flow synced successfully"
)

// Controller is the controller implementation for Flow resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// flowclientset is a clientset for our own API group
	flowclientset clientset.Interface

	//flowsLister listers.FlowLister
	flowsSynced cache.InformerSynced

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

// NewController returns a new flow controller
func NewController(
	kubeclientset kubernetes.Interface,
	flowclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	flowInformerFactory informers.SharedInformerFactory,
	routeInformerFactory elainformers.SharedInformerFactory) controller.Interface {

	// obtain a reference to a shared index informer for the Flow types.
	//flowInformer := flowInformerFactory.Eventing().V1beta2()

	// obtain a reference to a shared index informer for the Route type.
	routeInformer := routeInformerFactory.Elafros().V1alpha1().Routes()

	// Create event broadcaster
	// Add flow-controller types to the default Kubernetes Scheme so Events can be
	// logged for flow-controller types.
	flowscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset: kubeclientset,
		flowclientset: flowclientset,
		//flowsLister:   flowInformer.Flows().Lister(),
		//flowsSynced:  flowInformer.Flows().Informer().HasSynced,
		routesLister: routeInformer.Lister(),
		routesSynced: routeInformer.Informer().HasSynced,
		workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Flows"),
		recorder:     recorder,
	}

	controller.triggers = make(map[string]triggers.Trigger)
	controller.triggers["github"] = triggers.NewGithubTrigger()
	glog.Info("Setting up event handlers")
	// Set up an event handler for when Flow resources change
	/*flowInformer.Flows().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueFlow,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueFlow(new)
		},
	})*/

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
	glog.Info("Starting Flow controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for Flow informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.flowsSynced); !ok {
		return fmt.Errorf("failed to wait for Flow caches to sync")
	}

	glog.Info("Waiting for route informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.routesSynced); !ok {
		return fmt.Errorf("failed to wait for Route caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Flow resources
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
		// Flow resource to be synced.
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

// enqueueFlow takes a Flow resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Flow.
func (c *Controller) enqueueFlow(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Flow resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	/*namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}*/

	// Get the Flow resource with this namespace/name
	/*flow, err := c.flowsLister.Flows(namespace).Get(name)
	if err != nil {
		// The Flow resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("flow '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}
	// Don't mutate the informer's copy of our object.
	flow = flow.DeepCopy()*/

	// Find the Route that they want.
	/*routeName := flow.Spec.Action.RouteName
	route, err := c.routesLister.Routes(namespace).Get(routeName)
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("Route %q in namespace %q does not exist", routeName, namespace))
		}
		return err
	}
	glog.Infof("Found route url as '%q'", route.Spec.DomainSuffix)*/

	/*params, err := getParameters(c.kubeclientset, namespace, flow.Spec.Parameters.Raw, flow.Spec.ParametersFrom)
	if err != nil {
		glog.Warningf("Failed to process parameters: %s", err)
		return err
	}*/

	/*if flow.Status.Conditions != nil {
		glog.Infof("Already has status, skipping")
		return nil
	}*/

	/*glog.Infof("Creating a subscription to %q : %q with Parameters %+v", es.Name, et.Name, params)
	if val, ok := c.triggers[es.Name]; ok {
		r, err := val.Flow(params, route.Spec.DomainSuffix)
		if err != nil {
			glog.Warningf("FLOW failed: %s", err)
			msg := fmt.Sprintf("Flow failed with : %s", r)
			flow.Status.SetCondition(&v1beta2.FlowCondition{
				Type:    v1beta2.FlowFailed,
				Status:  corev1.ConditionTrue,
				Reason:  "FlowFailed",
				Message: msg,
			})
		} else {
			flow.Status.SetCondition(&v1beta2.FlowCondition{
				Type:    v1beta2.FlowComplete,
				Status:  corev1.ConditionTrue,
				Reason:  fmt.Sprintf("FlowSuccess: Hook: %s", r["ID"].(string)),
				Message: "Flow successful",
			})
		}
	}
	_, err = c.updateStatus(flow)
	if err != nil {
		glog.Warningf("Failed to update status: %s", err)
		return err
	}

	c.recorder.Event(flow, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	*/
	return nil
}

func (c *Controller) updateStatus(u *v1beta2.Flow) (*v1beta2.Flow, error) {
	flowClient := c.flowclientset.EventingV1beta2().Flows(u.Namespace)
	newu, err := flowClient.Get(u.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	newu.Status = u.Status

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Flow resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	return flowClient.Update(newu)
}

/*func getParameters(kubeClient kubernetes.Interface, namespace string, parameters []byte, parametersFrom []v1beta2.ParametersFromSource) (map[string]interface{}, error) {
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
}*/

/*func fetchParametersFromSource(kubeClient kubernetes.Interface, namespace string, parametersFrom *v1beta2.ParametersFromSource) (map[string]interface{}, error) {
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
}*/

/*func fetchSecretKeyValue(kubeClient kubernetes.Interface, namespace string, secretKeyRef *v1beta2.SecretKeyReference) ([]byte, error) {
	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(secretKeyRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return secret.Data[secretKeyRef.Key], nil
}*/

func unmarshalJSON(in []byte) (map[string]interface{}, error) {
	parameters := make(map[string]interface{})
	if err := json.Unmarshal(in, &parameters); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parameters as JSON object: %v", err)
	}
	return parameters, nil
}
