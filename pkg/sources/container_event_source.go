/*
Copyright 2018 Google, Inc. All rights reserved.

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

package sources

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	v1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ContainerEventSource struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	// Namespace to run the container in
	Namespace string

	// Binding for this operation
	Binding *v1alpha1.Bind

	// EventSourceSpec for the actual underlying source
	EventSourceSpec *v1alpha1.EventSourceSpec
}

func NewContainerEventSource(bind *v1alpha1.Bind, kubeclientset kubernetes.Interface, spec *v1alpha1.EventSourceSpec, namespace string) EventSource {
	return &ContainerEventSource{
		kubeclientset:   kubeclientset,
		Namespace:       namespace,
		Binding:         bind,
		EventSourceSpec: spec,
	}
}

func (t *ContainerEventSource) Bind(trigger EventTrigger, route string) (*BindContext, error) {
	pod, err := MakePod(t.Binding, t.Namespace, "bind-controller", "binder", t.EventSourceSpec, Bind, trigger, route, BindContext{})
	if err != nil {
		glog.Infof("failed to make pod: %s", err)
		return nil, err
	}
	bc, err := t.run(pod, true)
	if err != nil {
		glog.Infof("failed to bind: %s", err)
	}
	// Try to delete the pod even it failed to run
	delErr := t.delete(pod)
	if delErr != nil {
		glog.Infof("failed to delete the pod after running: %s", delErr)
	}
	return bc, err
}

func (t *ContainerEventSource) Unbind(trigger EventTrigger, bindContext BindContext) error {
	pod, err := MakePod(t.Binding, t.Namespace, "bind-controller", "binder", t.EventSourceSpec, Unbind, trigger, "", bindContext)
	if err != nil {
		glog.Infof("failed to make pod: %s", err)
		return err
	}
	_, err = t.run(pod, false)
	if err != nil {
		glog.Infof("failed to unbind pod: %s", err)
	}
	// Try to delete the pod anyways
	delErr := t.delete(pod)
	if delErr != nil {
		glog.Infof("failed to delete the pod after running: %s", delErr)
	}
	return err
}

func (t *ContainerEventSource) run(pod *v1.Pod, parseLogs bool) (*BindContext, error) {
	p := t.kubeclientset.CoreV1().Pods(pod.ObjectMeta.Namespace)
	_, err := p.Create(pod)
	if err != nil {
		return &BindContext{}, err
	}

	// TODO replace with a construct similar to Build by watching for pod
	// notifications and use channels for unblocking.
	for {
		time.Sleep(300 * time.Millisecond)
		glog.Infof("Checking pod...")
		myPod, err := p.Get(pod.ObjectMeta.Name, meta_v1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if myPod.Status.Phase == v1.PodFailed {
			glog.Infof("event source pod failed: %s", err)
			return nil, fmt.Errorf("Pod failed")
		}
		if myPod.Status.Phase == v1.PodSucceeded {
			glog.Infof("Pod Succeeded")
			for _, cs := range myPod.Status.ContainerStatuses {
				if cs.State.Terminated != nil && cs.State.Terminated.Message != "" {
					decodedContext, _ := base64.StdEncoding.DecodeString(cs.State.Terminated.Message)
					glog.Infof("Decoded to %q", decodedContext)
					var ret BindContext
					err = json.Unmarshal(decodedContext, &ret)
					if err != nil {
						glog.Infof("Failed to unmarshal context: %s", err)
						return nil, err
					}
					return &ret, nil
				}
			}
			return &BindContext{}, nil
		}
	}
}

func (t *ContainerEventSource) delete(pod *v1.Pod) error {
	p := t.kubeclientset.CoreV1().Pods(pod.ObjectMeta.Namespace)
	err := p.Delete(pod.ObjectMeta.Name, &meta_v1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			glog.Infof("Pod has already been deleted")
			return nil
		}
		glog.Infof("failed to delete pod: %s", err)
	}
	return err
}
