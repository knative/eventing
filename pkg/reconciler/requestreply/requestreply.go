/*
Copyright 2025 The Knative Authors

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

package requestreply

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/ptr"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	eventingv1listers "knative.dev/eventing/pkg/client/listers/eventing/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/network"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
)

const (
	secretName                  = "request-reply-keys"
	statefulSetName             = "request-reply"
	serviceName                 = "request-reply"
	triggerNameLabelKey         = "eventing.knative.dev/RequestReply.name"
	triggerNamespaceLabelKey    = "eventing.knative.dev/RequestReply.namespace"
	triggerReplicaCount         = "eventing.knative.dev/RequestReply.dataPlaneReplicas"
	triggerPreviousReplicaCount = "eventing.knative.dev/RequestReply.previousDataPlaneReplicas"
	SecretLabelSelector         = "eventing.knative.dev/part-of=request-reply"
)

type Reconciler struct {
	kubeClient        kubernetes.Interface
	eventingClient    clientset.Interface
	secretLister      corev1listers.SecretLister
	triggerLister     eventingv1listers.TriggerLister
	brokerLister      eventingv1listers.BrokerLister
	statefulSetLister appsv1listers.StatefulSetLister
	deleteContext     context.Context // used to delete triggers in a async cleanup operation
}

func (r *Reconciler) ReconcileKind(ctx context.Context, rr *v1alpha1.RequestReply) reconciler.Event {
	// 1. Ensure AES secret exists
	// 2. Check if all triggers to the data plane are created & ready
	// 4. Set address and ready

	err := r.reconcileAesSecret(ctx, rr)
	if err != nil {
		logging.FromContext(ctx).Errorf("failed to reconcile aes secret for requestreply", zap.Any("ReqestReply", rr), zap.Error(err))
		return fmt.Errorf("failed to reconcile aes secret: %w", err)
	}

	err = r.reconcileTriggers(ctx, rr)
	if err != nil {
		logging.FromContext(ctx).Errorf("failed to reconcile triggers for requestreply", zap.Any("RequestReply", rr), zap.Error(err))
		return fmt.Errorf("failed to reconcile triggers: %w", err)
	}

	err = r.reconcileBrokerAddressAnnotation(ctx, rr)
	if err != nil {
		logging.FromContext(ctx).Errorf("failed to reconcile broker address for requestreply", zap.Any("RequestReply", rr), zap.Error(err))
		return fmt.Errorf("failed to reconcile broker address: %w", err)
	}

	r.reconcileAddress(ctx, rr)

	rr.Status.MarkEventPoliciesTrueWithReason("NotImplemented", "Event policies not implemented for RequestReply yet")
	return nil
}

// reconcileAesSecret ensures that there is a AES secret for the current RequestReply resource
// TODO: we should rotate these somehow
func (r *Reconciler) reconcileAesSecret(ctx context.Context, rr *v1alpha1.RequestReply) error {
	// we want to ensure the secret exists
	secret, err := r.secretLister.Secrets(system.Namespace()).Get(secretName)
	if apierrs.IsNotFound(err) {
		aesKey, err := createAesSecret()
		if err != nil {
			return fmt.Errorf("failed to create new aes secret key: %w", err)
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: system.Namespace(),
			},
			Data: map[string][]byte{
				aesSecretKey(rr, 0): aesKey,
			},
		}

		_, err = r.kubeClient.CoreV1().Secrets(system.Namespace()).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create %s secret: %w", secretName, err)
		}
	}

	foundKey := false
	for k := range secret.Data {
		if keyIsForRequestReply(k, rr) {
			foundKey = true
			break
		}
	}

	if !foundKey {
		aesKey, err := createAesSecret()
		if err != nil {
			return fmt.Errorf("failed to create new aes secret key: %w", err)
		}

		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}

		secret.Data[aesSecretKey(rr, 0)] = aesKey

		_, err = r.kubeClient.CoreV1().Secrets(system.Namespace()).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update %s secret: %w", secretName, err)
		}
	}

	return nil
}

func (r *Reconciler) reconcileTriggers(ctx context.Context, rr *v1alpha1.RequestReply) error {
	ss, err := r.statefulSetLister.StatefulSets(system.Namespace()).Get(statefulSetName)
	if err != nil {
		return fmt.Errorf("failed to get requestreply statefulset: %w", err)
	}

	if rr.Status.Annotations == nil {
		rr.Status.Annotations = make(map[string]string)
	}

	if rr.Status.DesiredReplicas == nil {
		rr.Status.DesiredReplicas = ss.Spec.Replicas
	}

	if rr.Status.ReadyReplicas == nil {
		rr.Status.ReadyReplicas = ptr.To(int32(0))
	}

	// we always want to have the same number of replicas as statefulset pods
	if *rr.Status.DesiredReplicas != *ss.Spec.Replicas {
		// track previous replica count, so we can delete them when we are ready
		rr.Status.Annotations[triggerPreviousReplicaCount] = strconv.Itoa(int(*rr.Status.DesiredReplicas))
		rr.Status.DesiredReplicas = ss.Spec.Replicas
	}

	readyCount := 0
	var triggerErrors error = nil
	for i := range *rr.Status.DesiredReplicas {
		desired, err := r.createTrigger(ctx, int(i), rr, ss)
		if err != nil {
			triggerErrors = errors.Join(triggerErrors, fmt.Errorf("failed to create desired trigger object: %w", err))
			continue
		}
		t, err := r.triggerLister.Triggers(rr.Namespace).Get(triggerName(rr, int(i), int(*rr.Status.DesiredReplicas)))
		if apierrs.IsNotFound(err) {
			t, err = r.eventingClient.EventingV1().Triggers(rr.Namespace).Create(ctx, desired, metav1.CreateOptions{})
			if err != nil && !apierrs.IsAlreadyExists(err) {
				triggerErrors = errors.Join(triggerErrors, fmt.Errorf("failed to create trigger: %w", err))
			}
		} else if err != nil {
			triggerErrors = errors.Join(triggerErrors, fmt.Errorf("failed to get trigger: %w", err))
		}

		ignoreFields := cmpopts.IgnoreFields(eventingv1.TriggerSpec{}, "Filter")
		if equal, err := kmp.SafeEqual(t.Spec, desired.Spec, ignoreFields); !equal || err != nil {
			t.Spec = desired.Spec
			t, err = r.eventingClient.EventingV1().Triggers(rr.Namespace).Update(ctx, t, metav1.UpdateOptions{})
		} else {
			// only check if it is ready if it was equal, otherwise it needs to re-reconcile
			if t.Status.IsReady() {
				readyCount++
			}
		}
	}

	if *rr.Status.DesiredReplicas != *rr.Status.ReadyReplicas {
		// previously, the desired replicas were not equal to the ready replicas, let's try to do some cleanup
		previousReplicaCount, ok := rr.Status.Annotations[triggerPreviousReplicaCount]
		if ok && readyCount == int(*rr.Status.DesiredReplicas) {
			// we only just got all new triggers ready -> lets clean up the old ones
			d, err := time.ParseDuration(*rr.Spec.Timeout)
			if err != nil {
				logging.FromContext(ctx).Error("failed to parse timeout, defaulting to 1 minute for trigger cleanup logic", zap.Error(err))
				d = time.Minute
			}

			selector := labels.SelectorFromSet(map[string]string{
				triggerNameLabelKey:      rr.Name,
				triggerNamespaceLabelKey: rr.Namespace,
				triggerReplicaCount:      previousReplicaCount,
			})

			// we want to wait for timeout + a little bit to ensure that everything has time to drain
			time.AfterFunc(d+(time.Second*20), func() {
				triggers, err := r.triggerLister.List(selector)
				if err != nil {
					logging.FromContext(ctx).Error("failed to list triggers to clean up, some triggers may not be cleaned up", zap.Error(err))
				}

				for _, t := range triggers {
					r.deleteTrigger(t)
				}
			})

			delete(rr.Status.Annotations, triggerPreviousReplicaCount)
		}
	}

	rr.Status.ReadyReplicas = ptr.To(int32(readyCount))

	if *rr.Status.ReadyReplicas == *rr.Status.DesiredReplicas {
		rr.Status.MarkTriggersReady()
	} else {
		if triggerErrors != nil {
			rr.Status.MarkTriggersNotReadyWithReason("TriggerReconcileError", "Encountered errors reconciling triggers: %s", triggerErrors.Error())
		} else {
			rr.Status.MarkTriggersNotReadyWithReason("TriggersNotReady", "not all triggers are ready yet")
		}
	}

	return triggerErrors
}

func (r *Reconciler) reconcileBrokerAddressAnnotation(_ context.Context, rr *v1alpha1.RequestReply) error {
	broker, err := r.brokerLister.Brokers(rr.Namespace).Get(rr.Spec.BrokerRef.Name)
	if err != nil {
		rr.Status.MarkBrokerUnknown("NotFound", "could not get broker: %s", err.Error())
	}

	if !broker.IsReady() {
		rr.Status.MarkBrokerNotReady("NotReady", "broker is not ready")
	}

	if rr.Status.Annotations == nil {
		rr.Status.Annotations = make(map[string]string)
	}

	rr.Status.Annotations[v1alpha1.RequestReplyBrokerAddressStatusAnnotationKey] = broker.Status.Address.URL.String()

	rr.Status.MarkBrokerReady()

	return nil
}

func (r *Reconciler) reconcileAddress(ctx context.Context, rr *v1alpha1.RequestReply) {
	ingressHost := network.GetServiceHostname(serviceName, system.Namespace())
	address := &duckv1.Addressable{
		Name: ptr.To("http"),
		URL:  apis.HTTP(ingressHost),
	}
	address.URL.Path = fmt.Sprintf("/%s/%s", rr.Namespace, rr.Name)

	rr.Status.SetAddress(address)
}

func (r *Reconciler) createTrigger(ctx context.Context, idx int, rr *v1alpha1.RequestReply, ss *appsv1.StatefulSet) (*eventingv1.Trigger, error) {
	replicaCount := int(*rr.Status.DesiredReplicas)
	podName := fmt.Sprintf("%s-%d", ss.Name, idx)
	pod, err := r.kubeClient.CoreV1().Pods(ss.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod in statefulset: %w", err)
	}

	replyPath, err := apis.ParseURL(fmt.Sprintf("http://%s:8080/%s/%s/reply", pod.Status.PodIP, rr.Namespace, rr.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to parse reply path to data plane pod: %w", err)
	}

	t := &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      triggerName(rr, idx, replicaCount),
			Namespace: rr.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(rr),
			},
			Labels: map[string]string{
				triggerNameLabelKey:      rr.Name,
				triggerNamespaceLabelKey: rr.Namespace,
				triggerReplicaCount:      strconv.Itoa(replicaCount),
			},
		},
		Spec: eventingv1.TriggerSpec{
			Broker: rr.Spec.BrokerRef.Name,
			Filters: []eventingv1.SubscriptionsAPIFilter{
				{
					CESQL: fmt.Sprintf(
						"KN_VERIFY_CORRELATION_ID(%s, \"%s\", \"%s\", \"%s\", %d, %d)",
						rr.Spec.ReplyAttribute,
						rr.Name,
						rr.Namespace,
						secretName,
						idx,
						replicaCount,
					),
				},
			},
			Subscriber: duckv1.Destination{
				URI: replyPath,
			},
		},
	}

	return t, nil
}

func (r *Reconciler) deleteTrigger(trigger *eventingv1.Trigger) {
	ctx, cancel := context.WithTimeout(r.deleteContext, time.Second*30)
	defer cancel()
	err := r.eventingClient.EventingV1().Triggers(trigger.Namespace).Delete(ctx, trigger.Name, metav1.DeleteOptions{})
	if err != nil {
		logging.FromContext(ctx).Error("failed to delete trigger")
	}
}

func createAesSecret() ([]byte, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)

	return key, err
}

func aesSecretKey(rr *v1alpha1.RequestReply, idx int) string {
	return fmt.Sprintf("%s.%s.key-%d", rr.Namespace, rr.Name, idx)
}

func keyIsForRequestReply(name string, rr *v1alpha1.RequestReply) bool {
	return strings.HasPrefix(name, fmt.Sprintf("%s.%s", rr.Namespace, rr.Name))
}

func triggerName(rr *v1alpha1.RequestReply, idx, totalReplicas int) string {
	return kmeta.ChildName(rr.Name, fmt.Sprintf("%d-%d", idx, totalReplicas))
}
