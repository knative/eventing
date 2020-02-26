/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package prober

import (
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/await"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
)

func (p *prober) waitForKServiceReady(name, namespace string) error {
	meta := resources.NewMetaResource(name, namespace, &servingType)
	return duck.WaitForResourceReady(p.client.Dynamic, meta)
}

func (p *prober) waitForKServiceScale(name, namespace string, satisfyScale func(int32) bool) error {
	return wait.PollImmediate(await.Interval, await.Timeout, func() (bool, error) {
		serving := p.client.Dynamic.Resource(servicesCR).Namespace(namespace)
		unstruct, err := serving.Get(name, metav1.GetOptions{})
		return p.isScaledTo(satisfyScale, unstruct, namespace, err)
	})
}

func (p *prober) isScaledTo(satisfyScale func(int32) bool, un *unstructured.Unstructured, namespace string, err error) (bool, error) {
	if k8serrors.IsNotFound(err) {
		// Return false as we are not done yet.
		// We swallow the error to keep on polling.
		// It should only happen if we wait for the auto-created resources, like default Broker.
		return false, nil
	} else if err != nil {
		// Return error to stop the polling.
		return false, err
	}

	content := un.UnstructuredContent()
	maybeStatus, ok := content["status"]
	if !ok {
		return false, nil
	}
	status := maybeStatus.(map[string]interface{})
	maybeTraffic, ok := status["traffic"]
	if !ok {
		return false, nil
	}
	traffic := maybeTraffic.([]interface{})
	if len(traffic) > 1 {
		return false, fmt.Errorf("traffic shouldn't be split to more then 1 revision: %v", traffic)
	}
	if len(traffic) == 0 {
		// continue to wait
		return false, nil
	}
	firstTraffic := traffic[0].(map[string]interface{})
	revisionName := firstTraffic["revisionName"].(string)
	deploymentName := fmt.Sprintf("%s-deployment", revisionName)

	var dep *appsv1.Deployment
	dep, err = p.client.Kube.Kube.AppsV1().Deployments(namespace).
		Get(deploymentName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		// Return false as we are not done yet.
		return false, nil
	} else if err != nil {
		// Return error to stop the polling.
		return false, err
	}
	return satisfyScale(dep.Status.ReadyReplicas), nil
}

func (p *prober) waitForTriggerReady(name, namespace string) error {
	meta := resources.NewMetaResource(name, namespace, lib.TriggerTypeMeta)
	return duck.WaitForResourceReady(p.client.Dynamic, meta)
}

func (p *prober) waitForPodReady(name, namespace string) error {
	podType := &metav1.TypeMeta{
		Kind:       "Pod",
		APIVersion: "v1",
	}
	meta := resources.NewMetaResource(name, namespace, podType)
	return duck.WaitForResourceReady(p.client.Dynamic, meta)
}

type namedAwait struct {
	name    string
	routine awaitRoutine
}

type awaitRoutine func() error

var waits []*namedAwait

func awaitAll(log *zap.SugaredLogger) {
	var g errgroup.Group
	for _, w := range waits {
		w := w // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			log.Infof("Wait for %s", w.name)
			before := time.Now()
			err := w.routine()
			took := time.Now().Sub(before)
			if err != nil {
				log.Errorf("Error while waiting for %s: %v", w.name, err)
			} else {
				log.Infof("Successful wait for %s, took %v to complete", w.name, took)
			}
			return err
		})
	}
	waits = nil
	// Wait for all waits to complete.
	if err := g.Wait(); err != nil {
		panic(errors.New("there ware errors on waiting"))
	}
}

func waitFor(name string, routine awaitRoutine) {
	w := &namedAwait{
		name:    name,
		routine: routine,
	}
	waits = append(waits, w)
}
