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

package certificates

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/feature"
	cmclient "knative.dev/eventing/pkg/client/certmanager/clientset/versioned"
	cminformers "knative.dev/eventing/pkg/client/certmanager/informers/externalversions"
	cmlistersv1 "knative.dev/eventing/pkg/client/certmanager/listers/certmanager/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
)

const (
	SecretLabelKey   = "app.kubernetes.io/component"
	SecretLabelValue = "knative-eventing"

	CertificateLabelKey   = "app.kubernetes.io/component"
	CertificateLabelValue = "knative-eventing"

	SecretLabelSelectorPair      = SecretLabelKey + "=" + SecretLabelValue
	CertificateLabelSelectorPair = CertificateLabelKey + "=" + CertificateLabelValue
)

type DynamicCertificatesInformer struct {
	cancel atomic.Pointer[context.CancelFunc]
	lister atomic.Pointer[cmlistersv1.CertificateLister]
	mu     sync.Mutex
}

func NewDynamicCertificatesInformer() *DynamicCertificatesInformer {
	return &DynamicCertificatesInformer{
		cancel: atomic.Pointer[context.CancelFunc]{},
		lister: atomic.Pointer[cmlistersv1.CertificateLister]{},
		mu:     sync.Mutex{},
	}
}

func (df *DynamicCertificatesInformer) Reconcile(ctx context.Context, features feature.Flags, handler cache.ResourceEventHandler) error {
	df.mu.Lock()
	defer df.mu.Unlock()
	if !features.IsPermissiveTransportEncryption() && !features.IsStrictTransportEncryption() {
		return df.stop(ctx)
	}
	if df.cancel.Load() != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)

	client := cmclient.NewForConfigOrDie(injection.GetConfig(ctx))

	factory := cminformers.NewSharedInformerFactoryWithOptions(
		client,
		controller.DefaultResyncPeriod,
		cminformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = "app.kubernetes.io/component=knative-eventing"
		}),
	)
	informer := factory.Certmanager().V1().Certificates()
	informer.Informer().AddEventHandler(handler)

	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		defer cancel()
		return fmt.Errorf("failed to sync cert-manager Certificate informer")
	}

	lister := informer.Lister()
	df.lister.Store(&lister)
	df.cancel.Store(&cancel) // Cancel is always set as last field since it's used as a "guard".

	return nil
}

func (df *DynamicCertificatesInformer) stop(ctx context.Context) error {
	cancel := df.cancel.Load()
	if cancel == nil {
		logging.FromContext(ctx).Debugw("Certificate informer has not been started, nothing to stop")
		return nil
	}

	(*cancel)()
	df.lister.Store(nil)
	df.cancel.Store(nil) // Cancel is always set as last field since it's used as a "guard".
	return nil
}

func (df *DynamicCertificatesInformer) Lister() *atomic.Pointer[cmlistersv1.CertificateLister] {
	df.mu.Lock()
	defer df.mu.Unlock()

	return &df.lister
}

func CertificateName(objName string) string {
	return kmeta.ChildName(objName, "-server-tls")
}

type CertificateOption func(cert *cmv1.Certificate)

func MakeCertificate(obj kmeta.OwnerRefableAccessor, opts ...CertificateOption) *cmv1.Certificate {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CertificateName(obj.GetName()),
			Namespace: obj.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(obj),
			},
			Labels: map[string]string{
				CertificateLabelKey:      CertificateLabelValue,
				"app.kubernetes.io/name": obj.GetName(),
			},
		},
		Spec: cmv1.CertificateSpec{
			SecretName: CertificateName(obj.GetName()),
			SecretTemplate: &cmv1.CertificateSecretTemplate{
				Labels: map[string]string{
					SecretLabelKey:           SecretLabelValue,
					"app.kubernetes.io/name": obj.GetName(),
				},
			},
			Duration: &metav1.Duration{
				Duration: 90 * 24 * time.Hour, // 90 days
			},
			RenewBefore: &metav1.Duration{
				Duration: 15 * 24 * time.Hour, // 15 days
			},

			Subject: &cmv1.X509Subject{
				Organizations: []string{"local"},
			},
			PrivateKey: &cmv1.CertificatePrivateKey{
				Algorithm:      cmv1.RSAKeyAlgorithm,
				Encoding:       cmv1.PKCS1,
				Size:           2048,
				RotationPolicy: cmv1.RotationPolicyAlways,
			},
			IssuerRef: cmmeta.ObjectReference{
				Name:  "knative-eventing-ca-issuer",
				Kind:  "ClusterIssuer",
				Group: "cert-manager.io",
			},
		},
	}

	for _, opt := range opts {
		opt(cert)
	}
	return cert
}

func WithDNSNames(dnsNames ...string) CertificateOption {
	return func(cert *cmv1.Certificate) {
		cert.Spec.DNSNames = dnsNames
	}
}
