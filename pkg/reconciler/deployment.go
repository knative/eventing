/*
Copyright 2020 The Knative Authors

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
package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// newDeploymentCreated makes a new reconciler event with event type Normal, and
// reason DeploymentCreated.
func newDeploymentCreated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "DeploymentCreated", "created deployment: \"%s/%s\"", namespace, name)
}

// newDeploymentFailed makes a new reconciler event with event type Warning, and
// reason DeploymentFailed.
func newDeploymentFailed(namespace, name string, err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "DeploymentFailed", "failed to create deployment: \"%s/%s\", %w", namespace, name, err)
}

// newDeploymentUpdated makes a new reconciler event with event type Normal, and
// reason DeploymentUpdated.
func newDeploymentUpdated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "DeploymentUpdated", "updated deployment: \"%s/%s\"", namespace, name)
}

type Patcher func(deployment *appsv1.Deployment)

type DeploymentReconciler struct {
	kubeClientSet kubernetes.Interface
	template      *appsv1.Deployment
}

func NewDeploymentReconciler(ctx context.Context, template *appsv1.Deployment) DeploymentReconciler {
	return DeploymentReconciler{
		kubeClientSet: kubeclient.Get(ctx),
		template:      template,
	}
}

// ReconcileDeployment reconciles deployment resource
func (r *DeploymentReconciler) ReconcileDeployment(ctx context.Context, owner kmeta.OwnerRefable, patcher Patcher) (*appsv1.Deployment, pkgreconciler.Event) {
	expected := r.template.DeepCopy()
	if patcher != nil {
		patcher(expected)
	}

	expected.OwnerReferences = []metav1.OwnerReference{
		*kmeta.NewControllerRef(owner),
	}
	namespace := expected.Namespace
	name := expected.Name

	d, err := r.kubeClientSet.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		d, err = r.kubeClientSet.AppsV1().Deployments(namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return nil, newDeploymentFailed(namespace, name, err)
		}
		return d, newDeploymentCreated(d.Namespace, d.Name)
	} else if err != nil {
		return nil, fmt.Errorf("error getting deployment: %v", err)
	} else if !metav1.IsControlledBy(d, owner.GetObjectMeta()) {
		return nil, fmt.Errorf("deployment %s is not owned by %s/%s", name, owner.GetObjectMeta().GetNamespace(), owner.GetObjectMeta().GetName())
	} else if r.changed(d, expected) {
		b, err := json.Marshal(expected)
		if err != nil {
			return nil, newDeploymentFailed(namespace, name, err)
		}
		patchOptions := metav1.PatchOptions{
			Force:        pointer.BoolPtr(true),
			FieldManager: owner.GetObjectMeta().GetName(),
		}

		if d, err = r.kubeClientSet.AppsV1().Deployments(namespace).Patch(ctx, name, types.ApplyPatchType, b, patchOptions); err != nil {
			logging.FromContext(ctx).Errorw("failed to patch deployment", zap.Error(err))
			return d, err
		}
		logging.FromContext(ctx).Infow("deployment patched", zap.String("namespace", namespace), zap.String("name", name))
		return d, newDeploymentUpdated(namespace, name)
	} else {
		logging.FromContext(ctx).Debugw("reusing existing deployment", zap.Any("deployment", d))
	}
	return d, nil
}

func (r *DeploymentReconciler) changed(actual, expected *appsv1.Deployment) bool {
	return !equality.Semantic.DeepDerivative(expected.Annotations, actual.Annotations) ||
		!equality.Semantic.DeepDerivative(expected.Labels, actual.Labels) ||
		!equality.Semantic.DeepDerivative(expected.Spec, actual.Spec)
}

// ReadDeploymentFromKoData tries to read data as string from the file with given name
// under KO_DATA_PATH then returns the content as a Deploymnet. The file is expected
// to be wrapped into the container from /kodata by ko. If it fails, returns
// the error it gets.
func ReadDeploymentFromKoData(filename string) (*appsv1.Deployment, error) {
	koDataPath := os.Getenv("KO_DATA_PATH")
	if koDataPath == "" {
		return nil, fmt.Errorf("KO_DATA_PATH does not exist or is empty")
	}
	b, err := ioutil.ReadFile(filepath.Join(koDataPath, filename))
	if err != nil {
		return nil, err
	}

	var deployment appsv1.Deployment
	if err := yaml.Unmarshal(b, &deployment); err != nil {
		return nil, err
	}
	return &deployment, nil
}

// TODO: move to pkg

type ownerRefableDeployment appsv1.Deployment

func (o ownerRefableDeployment) GetGroupVersionKind() schema.GroupVersionKind {
	return appsv1.SchemeGroupVersion.WithKind("Deployment")
}

// DeploymentAsOwnerRefable returns a owner refable deployment
func DeploymentAsOwnerRefable(d *appsv1.Deployment) kmeta.OwnerRefable {
	return (*ownerRefableDeployment)(d)
}
