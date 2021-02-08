/*
Copyright 2021 The Knative Authors

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

package sample

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	fakesecretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/factory/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	pkgreconciler "knative.dev/pkg/reconciler"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/control/certificates"
)

func setupTest(t *testing.T, ctor injection.ControllerConstructor) (context.Context, *controller.Impl) {
	ctx, cf, _ := SetupFakeContextWithCancel(t)
	t.Cleanup(cf)

	configMapWatcher := &configmap.ManualWatcher{Namespace: system.Namespace()}
	ctrl := ctor(ctx, configMapWatcher)

	// The Reconciler won't do any work until it becomes the leader.
	if la, ok := ctrl.Reconciler.(pkgreconciler.LeaderAware); ok {
		require.NoError(t, la.Promote(
			pkgreconciler.UniversalBucket(),
			func(pkgreconciler.Bucket, types.NamespacedName) {},
		))
	}
	return ctx, ctrl
}

func TestReconcile(t *testing.T) {
	// The key to use, which for this singleton reconciler doesn't matter (although the
	// namespace matters for namespace validation).
	namespace := system.Namespace()
	caSecretName := "my-ctrl-ca"
	labelName := "my-ctrl"

	caKP, caKey, caCertificate := mustCreateCACert(t, 10*time.Hour)

	wellFormedCaSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      caSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			certificates.SecretCertKey: caKP.CertBytes(),
			certificates.SecretPKKey:   caKP.PrivateKeyBytes(),
		},
	}

	controlPlaneKP := mustCreateControlPlaneCert(t, 10*time.Hour, caKey, caCertificate)

	wellFormedControlPlaneSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "control-plane-ctrl",
			Namespace: namespace,
			Labels: map[string]string{
				labelName: controlPlaneSecretType,
			},
		},
		Data: map[string][]byte{
			certificates.SecretCaCertKey: caKP.CertBytes(),
			certificates.SecretCertKey:   controlPlaneKP.CertBytes(),
			certificates.SecretPKKey:     controlPlaneKP.PrivateKeyBytes(),
		},
	}

	dataPlaneKP := mustCreateDataPlaneCert(t, 10*time.Hour, caKey, caCertificate)

	wellFormedDataPlaneSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-plane-ctrl",
			Namespace: namespace,
			Labels: map[string]string{
				labelName: dataPlaneSecretType,
			},
		},
		Data: map[string][]byte{
			certificates.SecretCaCertKey: caKP.CertBytes(),
			certificates.SecretCertKey:   dataPlaneKP.CertBytes(),
			certificates.SecretPKKey:     dataPlaneKP.PrivateKeyBytes(),
		},
	}

	tests := []struct {
		name    string
		ctor    injection.ControllerConstructor
		key     string
		objects []*corev1.Secret
		asserts map[types.NamespacedName]func(*corev1.Secret)
	}{{
		name:    "well formed secret CA and control plane secret exists",
		key:     namespace + "/control-plane-ctrl",
		ctor:    NewControllerFactory("my"),
		objects: []*corev1.Secret{wellFormedCaSecret, wellFormedControlPlaneSecret},
		asserts: map[types.NamespacedName]func(*corev1.Secret){
			types.NamespacedName{
				Namespace: namespace,
				Name:      wellFormedCaSecret.Name,
			}: func(secret *corev1.Secret) {
				require.Equal(t, wellFormedCaSecret, secret)
			},
			types.NamespacedName{
				Namespace: namespace,
				Name:      wellFormedControlPlaneSecret.Name,
			}: func(secret *corev1.Secret) {
				require.Equal(t, wellFormedControlPlaneSecret, secret)
			},
		},
	}, {
		name:    "well formed secret CA and data plane secret exists",
		key:     namespace + "/data-plane-ctrl",
		ctor:    NewControllerFactory("my"),
		objects: []*corev1.Secret{wellFormedCaSecret, wellFormedDataPlaneSecret},
		asserts: map[types.NamespacedName]func(*corev1.Secret){
			types.NamespacedName{
				Namespace: namespace,
				Name:      wellFormedCaSecret.Name,
			}: func(secret *corev1.Secret) {
				require.Equal(t, wellFormedCaSecret, secret)
			},
			types.NamespacedName{
				Namespace: namespace,
				Name:      wellFormedDataPlaneSecret.Name,
			}: func(secret *corev1.Secret) {
				require.Equal(t, wellFormedDataPlaneSecret, secret)
			},
		},
	}, {
		name: "well formed secret CA but empty control plane secret",
		key:  namespace + "/control-plane-ctrl",
		ctor: NewControllerFactory("my"),
		objects: []*corev1.Secret{wellFormedCaSecret, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane-ctrl",
				Namespace: namespace,
				Labels: map[string]string{
					labelName: controlPlaneSecretType,
				},
			},
		}},
		asserts: map[types.NamespacedName]func(*corev1.Secret){
			types.NamespacedName{
				Namespace: namespace,
				Name:      wellFormedCaSecret.Name,
			}: func(secret *corev1.Secret) {
				require.Equal(t, wellFormedCaSecret, secret)
			},
			types.NamespacedName{
				Namespace: namespace,
				Name:      "control-plane-ctrl",
			}: func(secret *corev1.Secret) {
				require.Contains(t, secret.Data, certificates.SecretCaCertKey)
				require.Contains(t, secret.Data, certificates.SecretPKKey)
				require.Contains(t, secret.Data, certificates.SecretCertKey)
			},
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, ctrl := setupTest(t, test.ctor)

			for _, s := range test.objects {
				_, err := fakekubeclient.Get(ctx).CoreV1().Secrets(s.Namespace).Create(ctx, s, metav1.CreateOptions{})
				require.NoError(t, err)
				require.NoError(t, fakesecretinformer.Get(ctx).Informer().GetIndexer().Add(s))
			}

			require.NoError(t, ctrl.Reconciler.Reconcile(context.Background(), test.key))

			for name, asserts := range test.asserts {
				sec, err := fakekubeclient.Get(ctx).CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
				require.NoError(t, err)
				asserts(sec)
			}
		})
	}
}

func mustCreateCACert(t *testing.T, expirationInterval time.Duration) (*certificates.KeyPair, *rsa.PrivateKey, *x509.Certificate) {
	kp, err := certificates.CreateCACerts(context.TODO(), expirationInterval)
	require.NoError(t, err)
	pk, cert, err := certificates.ParseCert(kp.CertBytes(), kp.PrivateKeyBytes())
	require.NoError(t, err)
	return kp, cert, pk
}

func mustCreateDataPlaneCert(t *testing.T, expirationInterval time.Duration, caKey *rsa.PrivateKey, caCertificate *x509.Certificate) *certificates.KeyPair {
	kp, err := certificates.CreateDataPlaneCert(context.TODO(), caKey, caCertificate, expirationInterval)
	require.NoError(t, err)
	return kp
}

func mustCreateControlPlaneCert(t *testing.T, expirationInterval time.Duration, caKey *rsa.PrivateKey, caCertificate *x509.Certificate) *certificates.KeyPair {
	kp, err := certificates.CreateControlPlaneCert(context.TODO(), caKey, caCertificate, expirationInterval)
	require.NoError(t, err)
	return kp
}
