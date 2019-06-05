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

package controller

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/knative/eventing/pkg/reconciler/names"
	"github.com/knative/pkg/apis"

	"github.com/knative/eventing/contrib/natss/pkg/apis/messaging/v1alpha1"
	clientset "github.com/knative/eventing/contrib/natss/pkg/client/clientset/versioned"
	messaginginformers "github.com/knative/eventing/contrib/natss/pkg/client/informers/externalversions/messaging/v1alpha1"
	listers "github.com/knative/eventing/contrib/natss/pkg/client/listers/messaging/v1alpha1"
	"github.com/knative/eventing/contrib/natss/pkg/reconciler/controller/resources"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "NatssChannels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "natss-ch-controller"

	finalizerName = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process.
	channelReconciled         = "ChannelReconciled"
	channelReconcileFailed    = "ChannelReconcileFailed"
	channelUpdateStatusFailed = "ChannelUpdateStatusFailed"
)

// Reconciler reconciles NATSS Channels.
type Reconciler struct {
	*reconciler.Base

	dispatcherNamespace      string
	dispatcherDeploymentName string
	dispatcherServiceName    string

	eventingClientSet    clientset.Interface
	natsschannelLister   listers.NatssChannelLister
	natsschannelInformer cache.SharedIndexInformer
	deploymentLister     appsv1listers.DeploymentLister
	serviceLister        corev1listers.ServiceLister
	endpointsLister      corev1listers.EndpointsLister
	impl                 *controller.Impl
}

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
	serviceGVK    = corev1.SchemeGroupVersion.WithKind("Service")
)

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// Check that our Reconciler implements cache.ResourceEventHandler
var _ cache.ResourceEventHandler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	opt reconciler.Options,
	eventingClientSet clientset.Interface,
	dispatcherNamespace string,
	dispatcherDeploymentName string,
	dispatcherServiceName string,
	natsschannelInformer messaginginformers.NatssChannelInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	serviceInformer corev1informers.ServiceInformer,
	endpointsInformer corev1informers.EndpointsInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:                     reconciler.NewBase(opt, controllerAgentName),
		dispatcherNamespace:      dispatcherNamespace,
		dispatcherDeploymentName: dispatcherDeploymentName,
		dispatcherServiceName:    dispatcherServiceName,
		eventingClientSet:        eventingClientSet,
		natsschannelLister:       natsschannelInformer.Lister(),
		natsschannelInformer:     natsschannelInformer.Informer(),
		deploymentLister:         deploymentInformer.Lister(),
		serviceLister:            serviceInformer.Lister(),
		endpointsLister:          endpointsInformer.Lister(),
	}
	r.impl = controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")
	natsschannelInformer.Informer().AddEventHandler(controller.HandleAll(r.impl.Enqueue))

	// Set up watches for dispatcher resources we care about, since any changes to these
	// resources will affect our Channels. So, set up a watch here, that will cause
	// a global Resync for all the channels to take stock of their health when these change.
	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(dispatcherNamespace, dispatcherDeploymentName),
		Handler:    r,
	})
	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(dispatcherNamespace, dispatcherServiceName),
		Handler:    r,
	})
	endpointsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(dispatcherNamespace, dispatcherServiceName),
		Handler:    r,
	})
	return r.impl
}

// cache.ResourceEventHandler implementation.
// These 3 functions just cause a Global Resync of the channels, because any changes here
// should be reflected onto the channels.
func (r *Reconciler) OnAdd(obj interface{}) {
	r.impl.GlobalResync(r.natsschannelInformer)
}

func (r *Reconciler) OnUpdate(old, new interface{}) {
	r.impl.GlobalResync(r.natsschannelInformer)
}

func (r *Reconciler) OnDelete(obj interface{}) {
	r.impl.GlobalResync(r.natsschannelInformer)
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the NatssChannel resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the NatssChannels resource with this namespace/name.
	original, err := r.natsschannelLister.NatssChannels(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("NatssChannel key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy.
	channel := original.DeepCopy()

	// Reconcile this copy of the NatssChannels and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, channel)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling NatssChannel", zap.Error(reconcileErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, channelReconcileFailed, "NatssChannel reconciliation failed: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("NatssChannel reconciled")
		r.Recorder.Event(channel, corev1.EventTypeNormal, channelReconciled, "NatssChannel reconciled")
	}

	if _, updateStatusErr := r.updateStatus(ctx, channel); updateStatusErr != nil {
		logging.FromContext(ctx).Error("Failed to update NatssChannel status", zap.Error(updateStatusErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, channelUpdateStatusFailed, "Failed to update NatssChannel's status: %v", updateStatusErr)
		return updateStatusErr
	}

	// Requeue if the resource is not ready
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, nc *v1alpha1.NatssChannel) error {
	nc.Status.InitializeConditions()

	logger := logging.FromContext(ctx)
	// Verify channel is valid.
	if err := nc.Validate(ctx); err != nil {
		logger.Error("Invalid natss channel", zap.String("channel", nc.Name), zap.Error(err))
		return err
	}

	// See if the channel has been deleted.
	if nc.DeletionTimestamp != nil {
		// K8s garbage collection will delete the K8s service for this channel.
		return nil
	}

	// We reconcile the status of the Channel by looking at:
	// 1. Dispatcher Deployment for it's readiness.
	// 2. Dispatcher k8s Service for it's existence.
	// 3. Dispatcher endpoints to ensure that there's something backing the Service.
	// 4. K8s service representing the channel that will use ExternalName to point to the Dispatcher k8s service.

	// Get the Dispatcher Deployment and propagate the status to the Channel
	d, err := r.deploymentLister.Deployments(r.dispatcherNamespace).Get(r.dispatcherDeploymentName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			nc.Status.MarkDispatcherFailed("DispatcherDeploymentDoesNotExist", "Dispatcher Deployment does not exist")
		} else {
			logger.Error("Unable to get the dispatcher Deployment", zap.Error(err))
			nc.Status.MarkDispatcherFailed("DispatcherDeploymentGetFailed", "Failed to get dispatcher Deployment")
		}
		return err
	}
	nc.Status.PropagateDispatcherStatus(&d.Status)

	// Get the Dispatcher Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	_, err = r.serviceLister.Services(r.dispatcherNamespace).Get(r.dispatcherServiceName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			nc.Status.MarkServiceFailed("DispatcherServiceDoesNotExist", "Dispatcher Service does not exist")
		} else {
			logger.Error("Unable to get the dispatcher service", zap.Error(err))
			nc.Status.MarkServiceFailed("DispatcherServiceGetFailed", "Failed to get dispatcher service")
		}
		return err
	}
	nc.Status.MarkServiceTrue()

	// Get the Dispatcher Service Endpoints and propagate the status to the Channel
	// endpoints has the same name as the service, so not a bug.
	e, err := r.endpointsLister.Endpoints(r.dispatcherNamespace).Get(r.dispatcherServiceName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			nc.Status.MarkEndpointsFailed("DispatcherEndpointsDoesNotExist", "Dispatcher Endpoints does not exist")
		} else {
			logger.Error("Unable to get the dispatcher endpoints", zap.Error(err))
			nc.Status.MarkEndpointsFailed("DispatcherEndpointsGetFailed", "Failed to get dispatcher endpoints")
		}
		return err
	}

	if len(e.Subsets) == 0 {
		logger.Error("No endpoints found for Dispatcher service", zap.Error(err))
		nc.Status.MarkEndpointsFailed("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service")
		return fmt.Errorf("there are no endpoints ready for Dispatcher service %s", r.dispatcherServiceName)
	}
	nc.Status.MarkEndpointsTrue()

	// Reconcile the k8s service representing the actual Channel. It points to the Dispatcher service via ExternalName
	svc, err := r.reconcileChannelService(ctx, nc)
	if err != nil {
		nc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprintf("Channel Service failed: %s", err))
		return err
	}
	nc.Status.MarkChannelServiceTrue()
	nc.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
	})

	// Ok, so now the Dispatcher Deployment & Service have been created, we're golden since the
	// dispatcher watches the Channel and where it needs to dispatch events to.
	return nil
}

func (r *Reconciler) reconcileChannelService(ctx context.Context, channel *v1alpha1.NatssChannel) (*corev1.Service, error) {
	logger := logging.FromContext(ctx)
	// Get the  Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	// We may change this name later, so we have to ensure we use proper addressable when resolving these.
	svc, err := r.serviceLister.Services(channel.Namespace).Get(resources.MakeChannelServiceName(channel.Name))
	if err != nil {
		if apierrs.IsNotFound(err) {
			svc, err = resources.MakeK8sService(channel, resources.ExternalService(r.dispatcherNamespace, r.dispatcherServiceName))
			if err != nil {
				logger.Error("Failed to create the channel service object", zap.Error(err))
				return nil, err
			}
			svc, err = r.KubeClientSet.CoreV1().Services(channel.Namespace).Create(svc)
			if err != nil {
				logger.Error("Failed to create the channel service", zap.Error(err))
				return nil, err
			}
			return svc, nil
		} else {
			logger.Error("Unable to get the channel service", zap.Error(err))
		}
		return nil, err
	}
	// Check to make sure that the NatssChannel owns this service and if not, complain.
	if !metav1.IsControlledBy(svc, channel) {
		return nil, fmt.Errorf("natsschannel: %s/%s does not own Service: %q", channel.Namespace, channel.Name, svc.Name)
	}
	return svc, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.NatssChannel) (*v1alpha1.NatssChannel, error) {
	kc, err := r.natsschannelLister.NatssChannels(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(kc.Status, desired.Status) {
		return kc, nil
	}

	becomesReady := desired.Status.IsReady() && !kc.Status.IsReady()

	// Don't modify the informers copy.
	existing := kc.DeepCopy()
	existing.Status = desired.Status

	new, err := r.eventingClientSet.MessagingV1alpha1().NatssChannels(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(new.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("NatssChannel %q became ready after %v", kc.Name, duration)
		if err := r.StatsReporter.ReportReady("Channel", kc.Namespace, kc.Name, duration); err != nil {
			r.Logger.Infof("Failed to record ready for NatssChannel %q: %v", kc.Name, err)
		}
	}
	return new, err
}
