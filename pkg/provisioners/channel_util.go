package provisioners

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/labels"

	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/controller"
	"github.com/knative/eventing/pkg/system"
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

const (
	FinalizerAlreadyPresent AddFinalizerResult = false
	FinalizerAdded          AddFinalizerResult = true
)

// AddFinalizer adds finalizerName to the Channel.
func AddFinalizer(c *eventingv1alpha1.Channel, finalizerName string) AddFinalizerResult {
	finalizers := sets.NewString(c.Finalizers...)
	if finalizers.Has(finalizerName) {
		return FinalizerAlreadyPresent
	}
	finalizers.Insert(finalizerName)
	c.Finalizers = finalizers.List()
	return FinalizerAdded
}

func RemoveFinalizer(c *eventingv1alpha1.Channel, finalizerName string) {
	finalizers := sets.NewString(c.Finalizers...)
	finalizers.Delete(finalizerName)
	c.Finalizers = finalizers.List()
}

func CreateK8sService(ctx context.Context, client runtimeClient.Client, c *eventingv1alpha1.Channel) (*corev1.Service, error) {
	getSvc := func() (*corev1.Service, error) {
		return getK8sService(ctx, client, c)
	}
	return createK8sService(ctx, client, getSvc, newK8sService(c))
}

func getK8sService(ctx context.Context, client runtimeClient.Client, c *eventingv1alpha1.Channel) (*corev1.Service, error) {
	list := &corev1.ServiceList{}
	opts := &runtimeClient.ListOptions{
		Namespace: c.Namespace,
		// TODO After the full release start selecting on new set of labels by using k8sServiceLabels(c)
		LabelSelector: labels.SelectorFromSet(k8sOldServiceLabels(c)),
		// TODO this is here because the fake client needs it. Remove this when it's no longer
		// needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
		},
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
	svc.Spec.ClusterIP = current.Spec.ClusterIP
	if !equality.Semantic.DeepDerivative(svc.Spec, current.Spec) ||
		!expectedLabelsPresent(current.ObjectMeta.Labels, svc.ObjectMeta.Labels) {
		current.Spec = svc.Spec
		current.ObjectMeta.Labels = addExpectedLabels(current.ObjectMeta.Labels, svc.ObjectMeta.Labels)
		err = client.Update(ctx, current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

func getVirtualService(ctx context.Context, client runtimeClient.Client, c *eventingv1alpha1.Channel) (*istiov1alpha3.VirtualService, error) {
	list := &istiov1alpha3.VirtualServiceList{}
	opts := &runtimeClient.ListOptions{
		Namespace: c.Namespace,
		// TODO After the full release start selecting on new set of labels by using virtualServiceLabels(c)
		LabelSelector: labels.SelectorFromSet(virtualOldServiceLabels(c)),
		// TODO this is here because the fake client needs it. Remove this when it's no longer
		// needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: istiov1alpha3.SchemeGroupVersion.String(),
				Kind:       "VirtualService",
			},
		},
	}

	err := client.List(ctx, opts, list)
	if err != nil {
		return nil, err
	}
	for _, vs := range list.Items {
		if metav1.IsControlledBy(&vs, c) {
			return &vs, nil
		}
	}

	return nil, k8serrors.NewNotFound(schema.GroupResource{}, "")
}

func CreateVirtualService(ctx context.Context, client runtimeClient.Client, channel *eventingv1alpha1.Channel, svc *corev1.Service) (*istiov1alpha3.VirtualService, error) {
	virtualService, err := getVirtualService(ctx, client, channel)

	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		virtualService = newVirtualService(channel, svc)
		err = client.Create(ctx, virtualService)
		if err != nil {
			return nil, err
		}
		return virtualService, nil
	} else if err != nil {
		return nil, err
	}

	// Update VirtualService if it has changed. This is possible since in version 0.2.0, the destinationHost in
	// spec.HTTP.Route for the dispatcher was changed from *-clusterbus to *-dispatcher. Even otherwise, this
	// reconciliation is useful for the future mutations to the object.
	expected := newVirtualService(channel, svc)
	if !equality.Semantic.DeepDerivative(expected.Spec, virtualService.Spec) ||
		!expectedLabelsPresent(virtualService.ObjectMeta.Labels, expected.ObjectMeta.Labels) {
		virtualService.Spec = expected.Spec
		virtualService.ObjectMeta.Labels = addExpectedLabels(virtualService.ObjectMeta.Labels, expected.ObjectMeta.Labels)
		err := client.Update(ctx, virtualService)
		if err != nil {
			return nil, err
		}
	}
	return virtualService, nil
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
func newK8sService(c *eventingv1alpha1.Channel) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: channelServiceName(c.ObjectMeta.Name),
			Namespace:    c.Namespace,
			Labels:       k8sServiceLabels(c),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Channel",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: PortName,
					Port: PortNumber,
				},
			},
		},
	}
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

func virtualServiceLabels(c *eventingv1alpha1.Channel) map[string]string {
	// Use the same labels as the K8s service.
	return k8sServiceLabels(c)
}

func virtualOldServiceLabels(c *eventingv1alpha1.Channel) map[string]string {
	// Use the same labels as the K8s service.
	return k8sOldServiceLabels(c)
}

// newVirtualService creates a new VirtualService for a Channel resource. It also sets the
// appropriate OwnerReferences on the resource so handleObject can discover the Channel resource
// that 'owns' it. As well as being garbage collected when the Channel is deleted.
func newVirtualService(channel *eventingv1alpha1.Channel, svc *corev1.Service) *istiov1alpha3.VirtualService {
	destinationHost := controller.ServiceHostName(channelDispatcherServiceName(channel.Spec.Provisioner.Name), system.Namespace)
	return &istiov1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: channelVirtualServiceName(channel.Name),
			Namespace:    channel.Namespace,
			Labels:       virtualServiceLabels(channel),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(channel, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Channel",
				}),
			},
		},
		Spec: istiov1alpha3.VirtualServiceSpec{
			Hosts: []string{
				controller.ServiceHostName(svc.Name, channel.Namespace),
				channelHostName(channel.Name, channel.Namespace),
			},
			Http: []istiov1alpha3.HTTPRoute{{
				Rewrite: &istiov1alpha3.HTTPRewrite{
					Authority: channelHostName(channel.Name, channel.Namespace),
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

func channelVirtualServiceName(channelName string) string {
	return fmt.Sprintf("%s-channel-", channelName)
}

func channelServiceName(channelName string) string {
	return fmt.Sprintf("%s-channel-", channelName)
}

func channelHostName(channelName, namespace string) string {
	return fmt.Sprintf("%s.%s.channels.cluster.local", channelName, namespace)
}
