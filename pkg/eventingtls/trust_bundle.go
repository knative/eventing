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
	"context"
	"encoding/json"
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
	// TrustBundleLabelSelector is the ConfigMap label selector for trust bundles.
	TrustBundleLabelSelector = "networking.knative.dev/trust-bundle=true"

	TrustBundleMountPath = "/knative-custom-certs"

	TrustBundleVolumeNamePrefix = "kne-bundle-"
)

var (
	// TrustBundleSelector is a selector for trust bundle ConfigMaps.
	TrustBundleSelector labels.Selector
)

func init() {
	var err error
	TrustBundleSelector, err = labels.Parse(TrustBundleLabelSelector)
	if err != nil {
		panic(err)
	}
}

type pair struct {
	SysCM  *corev1.ConfigMap
	UserCm *corev1.ConfigMap
}

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

	logger := logging.FromContext(ctx)

	state := make(map[string]pair, len(systemNamespaceBundles)+len(userNamespaceBundles))

	for _, cm := range systemNamespaceBundles {
		if p, ok := state[cm.Name]; !ok {
			state[cm.Name] = pair{SysCM: cm}
		} else {
			state[cm.Name] = pair{
				SysCM:  cm,
				UserCm: p.UserCm,
			}
		}
	}

	for _, cm := range userNamespaceBundles {
		if p, ok := state[cm.Name]; !ok {
			state[cm.Name] = pair{UserCm: cm}
		} else {
			state[cm.Name] = pair{
				SysCM:  p.SysCM,
				UserCm: cm,
			}
		}
	}

	for _, p := range state {

		if p.SysCM == nil {
			if err := deleteConfigMap(ctx, k8s, obj, p.UserCm); err != nil {
				return err
			}
			continue
		}

		expected := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        p.SysCM.Name,
				Namespace:   obj.GetNamespace(),
				Labels:      p.SysCM.Labels,
				Annotations: p.SysCM.Annotations,
			},
			Data:       p.SysCM.Data,
			BinaryData: p.SysCM.BinaryData,
		}

		if p.UserCm == nil {
			// Update owner references
			expected.OwnerReferences = withOwnerReferences(obj, gvk, []metav1.OwnerReference{})

			if err := createConfigMap(ctx, k8s, expected); err != nil {
				sBytes, _ := json.Marshal(state)
				logger.Debugw("PropagateTrustBundles", zap.String("state", string(sBytes)))
				return fmt.Errorf("%w\n%s", err, string(sBytes))
			}
			continue
		}

		// Update owner references
		expected.OwnerReferences = withOwnerReferences(obj, gvk, p.UserCm.OwnerReferences)

		if !equality.Semantic.DeepDerivative(expected, p.UserCm) {
			if err := updateConfigMap(ctx, k8s, expected); err != nil {
				sBytes, _ := json.Marshal(state)
				logger.Debugw("PropagateTrustBundles", zap.String("state", string(sBytes)))
				return fmt.Errorf("%w\n%s", err, string(sBytes))
			}
		}
	}
	return nil
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
	expectedOr := metav1.OwnerReference{
		APIVersion: sb.GroupVersionKind().GroupVersion().String(),
		Kind:       sb.GroupVersionKind().Kind,
		Name:       sb.GetName(),
	}
	// Only delete the ConfigMap if the object owns it
	for _, or := range cm.OwnerReferences {
		if equality.Semantic.DeepDerivative(expectedOr, or) {
			err := k8s.CoreV1().ConfigMaps(sb.GetNamespace()).Delete(ctx, cm.Name, metav1.DeleteOptions{
				TypeMeta: metav1.TypeMeta{},
				Preconditions: &metav1.Preconditions{
					UID: &cm.UID,
				},
			})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete ConfigMap %s/%s: %w", cm.Namespace, cm.Name, err)
			}

			return nil
		}
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
