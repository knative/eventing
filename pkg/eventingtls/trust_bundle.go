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

package eventingtls

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sort"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
)

const (
	// TrustBundleLabelKey is the label key for trust bundles configmaps.
	TrustBundleLabelKey = "networking.knative.dev/trust-bundle"
	// TrustBundleLabelValue is the label value for trust bundles configmaps.
	TrustBundleLabelValue = "true"
	// TrustBundleLabelSelector is the ConfigMap label selector for trust bundles.
	TrustBundleLabelSelector = TrustBundleLabelKey + "=" + TrustBundleLabelValue

	TrustBundleMountPath = "/knative-custom-certs"

	TrustBundleVolumeNamePrefix = "kne-bundle-"

	TrustBundleConfigMapNameSuffix = "kne-bundle"

	TrustBundleCombined = "kn-combined-" + TrustBundleConfigMapNameSuffix

	TrustBundleCombinedPemFile = TrustBundleCombined + ".pem"
)

var (
	// TrustBundleSelector is a selector for trust bundle ConfigMaps.
	TrustBundleSelector = labels.SelectorFromSet(map[string]string{
		TrustBundleLabelKey: TrustBundleLabelValue,
	})
)

// PropagateTrustBundles propagates Trust bundles ConfigMaps from the system.Namespace() to the
// obj namespace.
func PropagateTrustBundles(ctx context.Context, k8s kubernetes.Interface, trustBundleConfigMapLister corev1listers.ConfigMapLister, gvk schema.GroupVersionKind, obj kmeta.Accessor) error {

	systemNamespaceBundles, err := trustBundleConfigMapLister.ConfigMaps(system.Namespace()).List(TrustBundleSelector)
	if err != nil {
		return fmt.Errorf("failed to list trust bundle ConfigMaps in %q: %w", system.Namespace(), err)
	}

	userNamespaceBundles, err := trustBundleConfigMapLister.ConfigMaps(obj.GetNamespace()).List(TrustBundleSelector)
	if err != nil {
		return fmt.Errorf("failed to list trust bundles ConfigMaps in %q: %w", obj.GetNamespace(), err)
	}

	logging.FromContext(ctx).Debugw("Existing bundles",
		zap.Any("systemNamespaceBundles", systemNamespaceBundles),
		zap.Any("userNamespaceBundles", userNamespaceBundles),
	)

	var combinedBundle bytes.Buffer
	if err := combineValidTrustBundles(systemNamespaceBundles, &combinedBundle); err != nil {
		return err
	}

	type Pair struct {
		sysCM  *corev1.ConfigMap
		userCm *corev1.ConfigMap
	}

	state := make(map[string]Pair, len(systemNamespaceBundles)+len(userNamespaceBundles))

	for _, cm := range systemNamespaceBundles {
		name := userCMName(cm.Name)
		if p, ok := state[name]; !ok {
			state[name] = Pair{sysCM: cm.DeepCopy()}
		} else {
			state[name] = Pair{
				sysCM:  cm.DeepCopy(),
				userCm: p.userCm,
			}
		}
	}

	// After going through the systemNamespaceBundles, add TrustBundleCombined to the system namespace state, so that
	// it will be reconciled in the user namespace.
	// If it's present in the system namespace, we "override" it to the "latest" (cache-based) value.
	if combinedBundle.Len() > 0 {
		state[userCMName(TrustBundleCombined)] = Pair{
			sysCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      TrustBundleCombined,
					Namespace: system.Namespace(),
					Labels: map[string]string{
						TrustBundleLabelKey: TrustBundleLabelValue,
					},
				},
				BinaryData: map[string][]byte{
					TrustBundleCombinedPemFile: combinedBundle.Bytes(),
				},
			},
		}
	}

	for _, cm := range userNamespaceBundles {
		if p, ok := state[cm.Name]; !ok {
			state[cm.Name] = Pair{userCm: cm.DeepCopy()}
		} else {
			state[cm.Name] = Pair{
				sysCM:  p.sysCM,
				userCm: cm.DeepCopy(),
			}
		}
	}

	for _, p := range state {

		if p.sysCM == nil {

			expectedOr := metav1.OwnerReference{
				APIVersion: gvk.GroupVersion().String(),
				Kind:       gvk.Kind,
				Name:       obj.GetName(),
				UID:        obj.GetUID(),
			}

			for _, or := range p.userCm.OwnerReferences {
				// Only delete the ConfigMap if the object owns it
				if equality.Semantic.DeepDerivative(expectedOr, or) {
					if err := deleteConfigMap(ctx, k8s, obj, p.userCm); err != nil {
						return err
					}
				}
			}
			continue
		}

		expected := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        userCMName(p.sysCM.Name),
				Namespace:   obj.GetNamespace(),
				Labels:      p.sysCM.Labels,
				Annotations: p.sysCM.Annotations,
			},
			Data:       p.sysCM.Data,
			BinaryData: p.sysCM.BinaryData,
		}

		if p.userCm == nil {
			// Update owner references
			expected.OwnerReferences = withOwnerReferences(obj, gvk, []metav1.OwnerReference{})
			if err := createConfigMap(ctx, k8s, expected); err != nil {
				return err
			}
			continue
		}

		expected.Generation = p.userCm.Generation
		expected.ResourceVersion = p.userCm.ResourceVersion
		// Update owner references
		expected.OwnerReferences = withOwnerReferences(obj, gvk, p.userCm.OwnerReferences)

		if !equality.Semantic.DeepDerivative(expected, p.userCm) {
			if err := updateConfigMap(ctx, k8s, expected); err != nil {
				return err
			}
		}
	}
	return nil
}

// CombinedBundlePresent will check if PropagateTrustBundles and AddTrustBundleVolumes will add the TrustBundleCombined
// volume.
//
// This is useful to know before creating the data plane how to configure the runtime to read the combined bundle.
// For example, Node.js allows injecting an environment variable `NODE_EXTRA_CA_CERTS` to point to a PEM-formatted
// trust bundle file, and we would need to know if that combined bundle will be injected by SinkBinding.
func CombinedBundlePresent(trustBundleLister corev1listers.ConfigMapLister) (bool, error) {
	cms, err := trustBundleLister.ConfigMaps(system.Namespace()).List(TrustBundleSelector)
	if err != nil {
		return false, fmt.Errorf("failed to list trust bundles ConfigMaps in %q: %w", system.Namespace(), err)
	}
	var combinedBundle bytes.Buffer
	if err := combineValidTrustBundles(cms, &combinedBundle); err != nil {
		return false, err
	}
	return combinedBundle.Len() > 0, nil
}

func AddTrustBundleVolumes(trustBundleLister corev1listers.ConfigMapLister, obj kmeta.Accessor, pt *corev1.PodSpec) (*corev1.PodSpec, error) {
	cms, err := trustBundleLister.ConfigMaps(obj.GetNamespace()).List(TrustBundleSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to list trust bundles ConfigMaps in %q: %w", obj.GetNamespace(), err)
	}

	pt = pt.DeepCopy()
	sources := make([]corev1.VolumeProjection, 0, len(cms))
	for _, cm := range cms {
		sources = append(sources, corev1.VolumeProjection{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cm.Name,
				},
			},
		})
	}
	if len(sources) == 0 {
		return pt, nil
	}

	volumeName := fmt.Sprintf("%s%s", TrustBundleVolumeNamePrefix, "volume")
	vs := corev1.VolumeSource{
		Projected: &corev1.ProjectedVolumeSource{
			Sources: sources,
		},
	}

	found := false
	for i, v := range pt.Volumes {
		if v.Name == volumeName {
			found = true
			pt.Volumes[i].VolumeSource = vs
			break
		}
	}
	if !found {
		pt.Volumes = append(pt.Volumes, corev1.Volume{
			Name:         volumeName,
			VolumeSource: vs,
		})
	}

	for i := range pt.Containers {
		found = false
		for _, v := range pt.Containers[i].VolumeMounts {
			if v.Name == volumeName {
				found = true
				break
			}
		}
		if !found {
			pt.Containers[i].VolumeMounts = append(pt.Containers[i].VolumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				ReadOnly:  true,
				MountPath: TrustBundleMountPath,
			})
		}
	}

	for i := range pt.InitContainers {
		found = false
		for _, v := range pt.InitContainers[i].VolumeMounts {
			if v.Name == volumeName {
				found = true
				break
			}
		}
		if !found {
			pt.InitContainers[i].VolumeMounts = append(pt.InitContainers[i].VolumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				ReadOnly:  true,
				MountPath: TrustBundleMountPath,
			})
		}
	}

	return pt, nil
}

func combineValidTrustBundles(configMaps []*corev1.ConfigMap, combinedBundle *bytes.Buffer) error {
	for _, cm := range configMaps {

		// Ensure the combined bundle is always composed in the same order.
		keys := make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			data := cm.Data[key]
			pemData := []byte(data)

			// Read and validate each PEM block in the data
			for len(pemData) > 0 {
				var block *pem.Block
				block, pemData = pem.Decode(pemData)

				if block == nil {
					// No more PEM blocks or invalid PEM format
					break
				}

				// If the combined bundle would produce a value larger than the maximum ConfigMap size, truncate it.
				// This could be surprising, but we can't do much about it, perhaps depending on the feedback we could
				// return an error (individual bundles can be used instead in such cases, instead of failing and
				// requiring admin-level changes).
				//
				// The encoded value is larger, so leave some buffer space.
				if combinedBundle.Len()+len(block.Bytes) > 1000000 /* < 1Mib -> max CM size */ {
					break
				}

				if block.Type != "CERTIFICATE" {
					// Skip non-certificate PEM blocks
					continue
				}

				// Attempt to parse the certificate to validate it
				if _, err := x509.ParseCertificate(block.Bytes); err != nil {
					// Invalid certificate format
					continue
				}

				// Certificate is valid, add to combined output
				if err := pem.Encode(combinedBundle, block); err != nil {
					return fmt.Errorf("failed to encode valid certificate from %s/%s ConfigMap for key %q: %v", cm.Namespace, cm.Name, key, err)
				}
			}
		}
	}
	return nil
}

func withOwnerReferences(sb kmeta.Accessor, gvk schema.GroupVersionKind, references []metav1.OwnerReference) []metav1.OwnerReference {
	expected := metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Name:       sb.GetName(),
	}
	found := false
	for i := range references {
		if equality.Semantic.DeepDerivative(expected, references[i]) {
			references[i].UID = sb.GetUID()
			found = true
		}
	}

	if !found {
		expected.UID = sb.GetUID()
		references = append(references, expected)
	}

	sort.SliceStable(references, func(i, j int) bool { return references[i].Name < references[j].Name })
	return references
}

func deleteConfigMap(ctx context.Context, k8s kubernetes.Interface, sb kmeta.Accessor, cm *corev1.ConfigMap) error {
	err := k8s.CoreV1().ConfigMaps(sb.GetNamespace()).Delete(ctx, cm.Name, metav1.DeleteOptions{
		TypeMeta: metav1.TypeMeta{},
		Preconditions: &metav1.Preconditions{
			UID:             &cm.UID,
			ResourceVersion: &cm.ResourceVersion,
		},
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ConfigMap %s/%s: %w", cm.Namespace, cm.Name, err)
	}

	return nil
}

func updateConfigMap(ctx context.Context, k8s kubernetes.Interface, expected *corev1.ConfigMap) error {
	_, err := k8s.CoreV1().ConfigMaps(expected.Namespace).Update(ctx, expected, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap %s/%s: %w", expected.Namespace, expected.Name, err)
	}
	return nil
}

func createConfigMap(ctx context.Context, k8s kubernetes.Interface, expected *corev1.ConfigMap) error {
	_, err := k8s.CoreV1().ConfigMaps(expected.Namespace).Create(ctx, expected, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap %s/%s: %w", expected.Namespace, expected.Name, err)
	}
	return nil
}

func userCMName(name string) string {
	return kmeta.ChildName(name, TrustBundleConfigMapNameSuffix)
}
