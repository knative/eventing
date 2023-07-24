/*
Copyright 2023 The Knative Authors

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

package certificate

import (
	"bytes"
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection/clients/dynamicclient"

	"knative.dev/reconciler-test/pkg/feature"
)

var (
	certificateGVR = schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "certificates",
	}
)

type RotateCertificate struct {
	Certificate types.NamespacedName
}

// Rotate rotates a cert-manager issued certificate.
// The procedure follows the same process as the cert-manager command `cmctl renew <cert-name>`
// See also https://cert-manager.io/docs/usage/certificate/#actions-triggering-private-key-rotation
func Rotate(rotate RotateCertificate) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		before := getSecret(ctx, t, rotate)
		issueRotation(ctx, t, rotate)
		waitForRotation(ctx, t, rotate, before)
	}

}

func issueRotation(ctx context.Context, t feature.T, rotate RotateCertificate) {
	var lastErr error
	err := wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		err := rotateCertificate(ctx, rotate)
		if err == nil {
			return true, nil
		}
		lastErr = err

		// Retry on conflicts
		if apierrors.IsConflict(err) {
			return false, nil
		}

		return false, err
	})
	if err != nil {
		t.Fatal(err, lastErr)
	}
}

type Certificate struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec Spec `json:"spec"`

	Status Status `json:"status"`
}

type Spec struct {
	SecretName string `json:"secretName"`
}

// Status defines the observed state of Certificate
type Status struct {
	duckv1.Status `json:",inline"`
	// Copied from https://github.com/cert-manager/cert-manager/blob/master/pkg/apis/certmanager/v1/types_certificate.go
	LastFailureTime          *metav1.Time `json:"lastFailureTime,omitempty"`
	NotBefore                *metav1.Time `json:"notBefore,omitempty"`
	NotAfter                 *metav1.Time `json:"notAfter,omitempty"`
	RenewalTime              *metav1.Time `json:"renewalTime,omitempty"`
	Revision                 *int         `json:"revision,omitempty"`
	NextPrivateKeySecretName *string      `json:"nextPrivateKeySecretName,omitempty"`
	FailedIssuanceAttempts   *int         `json:"failedIssuanceAttempts,omitempty"`
}

func rotateCertificate(ctx context.Context, rotate RotateCertificate) error {
	dc := dynamicclient.Get(ctx).Resource(certificateGVR)

	obj, err := dc.
		Namespace(rotate.Certificate.Namespace).
		Get(ctx, rotate.Certificate.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	cert := &Certificate{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, cert); err != nil {
		return err
	}

	renewCertificate(cert)

	obj.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(cert)
	if err != nil {
		return err
	}

	_, err = dc.
		Namespace(rotate.Certificate.Namespace).
		UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func waitForRotation(ctx context.Context, t feature.T, rotate RotateCertificate, before *corev1.Secret) {
	keys := []string{"tls.key", "tls.crt"}
	err := wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		current := getSecret(ctx, t, rotate)
		for _, key := range keys {
			if bytes.Equal(before.Data[key], current.Data[key]) {
				t.Logf("Value for key %s is equal", key)
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		t.Errorf("Failed while waiting for Certificate rotation to happen: %v", err)
	}
}

func getSecret(ctx context.Context, t feature.T, rotate RotateCertificate) *corev1.Secret {
	obj, err := dynamicclient.Get(ctx).Resource(certificateGVR).
		Namespace(rotate.Certificate.Namespace).
		Get(ctx, rotate.Certificate.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get certificate %s/%s: %v", rotate.Certificate.Namespace, rotate.Certificate.Name, err)
	}

	cert := &Certificate{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, cert); err != nil {
		t.Fatal(err)
	}

	secret, err := kubeclient.Get(ctx).
		CoreV1().
		Secrets(rotate.Certificate.Namespace).
		Get(ctx, cert.Spec.SecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get secret %s/%s: %v", rotate.Certificate.Namespace, cert.Spec.SecretName, err)
	}

	return secret
}

// Adapted from:
// - https://github.com/cert-manager/cert-manager/blob/843deed22f563dbdcbbf71a9fc478609ee90cb8e/pkg/api/util/conditions.go#L165-L204
// - https://github.com/cert-manager/cert-manager/blob/843deed22f563dbdcbbf71a9fc478609ee90cb8e/cmd/ctl/pkg/renew/renew.go#L206-L214
func renewCertificate(c *Certificate) {

	newCondition := apis.Condition{
		Type:    apis.ConditionType("Issuing"),
		Status:  corev1.ConditionTrue,
		Reason:  "ManuallyTriggered",
		Message: "Certificate re-issuance manually triggered",
	}

	nowTime := metav1.NewTime(time.Now())
	newCondition.LastTransitionTime = apis.VolatileTime{Inner: nowTime}

	// Search through existing conditions
	for idx, cond := range c.Status.GetConditions() {
		// Skip unrelated conditions
		if cond.Type != newCondition.Type {
			continue
		}

		// If this update doesn't contain a state transition, we don't update
		// the conditions LastTransitionTime to Now()
		if cond.Status == newCondition.Status {
			newCondition.LastTransitionTime = cond.LastTransitionTime
		}

		// Overwrite the existing condition
		c.Status.Conditions[idx] = newCondition
		return
	}

	c.Status.SetConditions(append(c.Status.GetConditions(), newCondition))
}
