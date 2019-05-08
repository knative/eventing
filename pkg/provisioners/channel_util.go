package provisioners

import (
	"context"
	"fmt"
	"github.com/knative/pkg/kmeta"
	"strings"

	"k8s.io/apimachinery/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler/names"
	"github.com/knative/eventing/pkg/utils"
	"github.com/knative/pkg/system"
	"k8s.io/apimachinery/pkg/api/equality"
)

const (
	PortName   = "http"
	PortNumber = 80
	// EventingChannelLabel carries the name of knative's label for the channel
	EventingChannelLabel = "eventing.knative.dev/channel"
	// EventingProvisionerLabel carries the name of knative's label for the provisioner
	EventingProvisionerLabel = "eventing.knative.dev/provisioner"

	// TODO: Remove selection based on old labels ater the release

	// OldEventingChannelLabel carries the name of knative's old label for the channel
	OldEventingChannelLabel = "channel"
	// OldEventingProvisionerLabel carries the name of knative's old label for the provisioner
	OldEventingProvisionerLabel = "provisioner"
)

// AddFinalizerResult is used indicate whether a finalizer was added or already present.
type AddFinalizerResult bool

// RemoveFinalizerResult is used to indicate whether a finalizer was found and removed (FinalizerRemoved), or finalizer not found (FinalizerNotFound).
type RemoveFinalizerResult bool

const (
	FinalizerAlreadyPresent AddFinalizerResult    = false
	FinalizerAdded          AddFinalizerResult    = true
	FinalizerRemoved        RemoveFinalizerResult = true
	FinalizerNotFound       RemoveFinalizerResult = false
)

// AddFinalizer adds finalizerName to the Object.
func AddFinalizer(o metav1.Object, finalizerName string) AddFinalizerResult {
	finalizers := sets.NewString(o.GetFinalizers()...)
	if finalizers.Has(finalizerName) {
		return FinalizerAlreadyPresent
	}
	finalizers.Insert(finalizerName)
	o.SetFinalizers(finalizers.List())
	return FinalizerAdded
}

// RemoveFinalizer removes the finalizer(finalizerName) from the object(o) if the finalizer is present.
// Returns: - FinalizerRemoved, if the finalizer was found and removed.
//          - FinalizerNotFound, if the finalizer was not found.
func RemoveFinalizer(o metav1.Object, finalizerName string) RemoveFinalizerResult {
	finalizers := sets.NewString(o.GetFinalizers()...)
	if finalizers.Has(finalizerName) {
		finalizers.Delete(finalizerName)
		o.SetFinalizers(finalizers.List())
		return FinalizerRemoved
	}
	return FinalizerNotFound
}

// K8sServiceOption is a functional option that can modify the K8s Service in CreateK8sService
type K8sServiceOption func(*corev1.Service) error

// ExternalService is a functional option for CreateK8sService to create a K8s service of type ExternalName.
func ExternalService(c *eventingv1alpha1.Channel) K8sServiceOption {
	return func(svc *corev1.Service) error {
		svc.Spec = corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: names.ServiceHostName(channelDispatcherServiceName(c.Spec.Provisioner.Name), system.Namespace()),
		}
		return nil
	}
}

func CreateK8sService(ctx context.Context, client runtimeClient.Client, c *eventingv1alpha1.Channel, opts ...K8sServiceOption) (*corev1.Service, error) {
	getSvc := func() (*corev1.Service, error) {
		return getK8sService(ctx, client, c)
	}
	svc, err := newK8sService(c, opts...)
	if err != nil {
		return nil, err
	}
	return createK8sService(ctx, client, getSvc, svc)
}

func getK8sService(ctx context.Context, client runtimeClient.Client, c *eventingv1alpha1.Channel) (*corev1.Service, error) {
	list := &corev1.ServiceList{}
	opts := &runtimeClient.ListOptions{
		Namespace:     c.Namespace,
		LabelSelector: labels.SelectorFromSet(k8sServiceLabels(c)),
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
	}

	err := client.List(ctx, opts, list)
	if err != nil {
		return nil, err
	}
	for _, svc := range list.Items {
		if metav1.IsControlledBy(&svc, c) {
			return &svc, nil
		}
	}

	return nil, k8serrors.NewNotFound(schema.GroupResource{}, "")
}

type getService func() (*corev1.Service, error)

func createK8sService(ctx context.Context, client runtimeClient.Client, getSvc getService, svc *corev1.Service) (*corev1.Service, error) {
	current, err := getSvc()
	if k8serrors.IsNotFound(err) {
		err = client.Create(ctx, svc)
		if err != nil {
			return nil, err
		}
		return svc, nil
	} else if err != nil {
		return nil, err
	}
	// spec.clusterIP is immutable and is set on existing services. If we don't set this
	// to the same value, we will encounter an error while updating.
	if svc.Spec.Type != corev1.ServiceTypeExternalName {
		svc.Spec.ClusterIP = current.Spec.ClusterIP
	}
	if !equality.Semantic.DeepDerivative(svc.Spec, current.Spec) ||
		!expectedLabelsPresent(current.ObjectMeta.Labels, svc.ObjectMeta.Labels) ||
		// This DeepEqual is necessary to force update dispatcher services when upgrading from 0.5 to 0.6.
		// Above DeepDerivative will not work because we have removed an optional field (name) from ports
		// TODO: Remove this check in 0.7+
		!equality.Semantic.DeepEqual(svc.Spec.Ports, current.Spec.Ports) {
		current.Spec = svc.Spec
		current.ObjectMeta.Labels = addExpectedLabels(current.ObjectMeta.Labels, svc.ObjectMeta.Labels)
		err = client.Update(ctx, current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// checkExpectedLabels checks the presence of expected labels and its values and return true
// if all labels are found.
func expectedLabelsPresent(actual, expected map[string]string) bool {
	for ke, ve := range expected {
		if va, ok := actual[ke]; ok {
			if strings.Compare(ve, va) == 0 {
				continue
			}
		}
		return false
	}
	return true
}

// addExpectedLabels adds expected labels
func addExpectedLabels(actual, expected map[string]string) map[string]string {
	consolidated := make(map[string]string, 0)
	// First store all exisiting labels
	for k, v := range actual {
		consolidated[k] = v
	}
	// Second add all missing expected labels
	for k, v := range expected {
		consolidated[k] = v
	}
	return consolidated
}

func UpdateChannel(ctx context.Context, client runtimeClient.Client, u *eventingv1alpha1.Channel) error {
	objectKey := runtimeClient.ObjectKey{Namespace: u.Namespace, Name: u.Name}
	channel := &eventingv1alpha1.Channel{}

	if err := client.Get(ctx, objectKey, channel); err != nil {
		return err
	}

	channelChanged := false

	if !equality.Semantic.DeepEqual(channel.Finalizers, u.Finalizers) {
		channel.SetFinalizers(u.ObjectMeta.Finalizers)
		if err := client.Update(ctx, channel); err != nil {
			return err
		}
		channelChanged = true
	}

	if equality.Semantic.DeepEqual(channel.Status, u.Status) {
		return nil
	}

	if channelChanged {
		// Refetch
		channel = &eventingv1alpha1.Channel{}
		if err := client.Get(ctx, objectKey, channel); err != nil {
			return err
		}
	}

	channel.Status = u.Status

	if err := client.Status().Update(ctx, channel); err != nil {
		return err
	}

	return nil
}

// newK8sService creates a new Service for a Channel resource. It also sets the appropriate
// OwnerReferences on the resource so handleObject can discover the Channel resource that 'owns' it.
// As well as being garbage collected when the Channel is deleted.
func newK8sService(c *eventingv1alpha1.Channel, opts ...K8sServiceOption) (*corev1.Service, error) {
	// Add annotations
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: channelServiceName(c.ObjectMeta.Name),
			Namespace:    c.Namespace,
			Labels:       k8sServiceLabels(c),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(c),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     PortName,
					Protocol: corev1.ProtocolTCP,
					Port:     PortNumber,
				},
			},
		},
	}
	for _, opt := range opts {
		if err := opt(svc); err != nil {
			return nil, err
		}
	}
	return svc, nil
}

// k8sOldServiceLabels returns a map with only old eventing channel and provisioner labels
func k8sOldServiceLabels(c *eventingv1alpha1.Channel) map[string]string {
	return map[string]string{
		OldEventingChannelLabel:     c.Name,
		OldEventingProvisionerLabel: c.Spec.Provisioner.Name,
	}
}

// k8sServiceLabels returns a map with eventing channel and provisioner labels
func k8sServiceLabels(c *eventingv1alpha1.Channel) map[string]string {
	return map[string]string{
		EventingChannelLabel:        c.Name,
		OldEventingChannelLabel:     c.Name,
		EventingProvisionerLabel:    c.Spec.Provisioner.Name,
		OldEventingProvisionerLabel: c.Spec.Provisioner.Name,
	}
}

func channelServiceName(channelName string) string {
	return fmt.Sprintf("%s-channel-", channelName)
}

func channelHostName(channelName, namespace string) string {
	return fmt.Sprintf("%s.%s.channels.%s", channelName, namespace, utils.GetClusterDomainName())
}

// NewHostNameToChannelRefMap parses each channel from cList and creates a map[string(Status.Address.HostName)]ChannelReference
func NewHostNameToChannelRefMap(cList []eventingv1alpha1.Channel) (map[string]ChannelReference, error) {
	hostToChanMap := make(map[string]ChannelReference, len(cList))
	for _, c := range cList {
		hostName := c.Status.Address.Hostname
		if cr, ok := hostToChanMap[hostName]; ok {
			return nil, fmt.Errorf(
				"Duplicate hostName found. Each channel must have a unique host header. HostName:%s, channel:%s.%s, channel:%s.%s",
				hostName,
				c.Namespace,
				c.Name,
				cr.Namespace,
				cr.Name)
		}
		hostToChanMap[hostName] = ChannelReference{Name: c.Name, Namespace: c.Namespace}
	}
	return hostToChanMap, nil
}
