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
package cache

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
)

const (
	// ResourcesKey is the configmap key holding resources
	ResourcesKey = "resources.json"

	// ComponentLabelKey is the label added to ConfigMaps
	ComponentLabelKey = "eventing.knative.dev/component"
)

// PersistedStore persists cache.Store objects to a single ConfigMap.
type PersistedStore interface {

	// Run starts persisting the cache store to the configmap-backed store
	Run(ctx context.Context)
}

// Projector projects the given object to the persisted object
type Projector func(interface{}) interface{}

type persistedStore struct {
	// Component associated to this store
	component string

	// kubeClient is the client use to manage kube objects
	kubeClient kubernetes.Interface

	// namespace of the configmap
	namespace string

	// name of the configmap
	name string

	// store is the in-memory cache
	store cache.Store

	// Projector function transforms objects before being persisted
	// If the returned object is nil, it is not persisted.
	project func(interface{}) interface{}

	// sync channel
	syncCh chan bool

	// syncing state (idle: 0, syncing: 1)
	syncing int32
}

// NewPersistedStore creates a persisted store for the given informer
func NewPersistedStore(
	component string,
	kubeClient kubernetes.Interface,
	namespace string,
	name string,
	informer cache.SharedInformer,
	projector Projector) PersistedStore {

	if projector == nil {
		// identity projection
		projector = func(v interface{}) interface{} {
			return v
		}
	}

	pstore := &persistedStore{
		component:  component,
		kubeClient: kubeClient,
		namespace:  namespace,
		name:       name,
		store:      informer.GetStore(),
		project:    projector,
		syncCh:     make(chan bool, 1),
		syncing:    0,
	}
	informer.AddEventHandler(pstore)

	return pstore
}

func (p *persistedStore) Run(ctx context.Context) {
	logger := logging.FromContext(ctx)

	// Trigger a sync to make sure the ConfigMap resource exists
	p.sync()

	wait.UntilWithContext(ctx, func(ctx context.Context) {
		select {
		case <-ctx.Done():
			return
		case <-p.syncCh:
			atomic.StoreInt32(&p.syncing, 1)
			if err := p.doSync(ctx.Done()); err != nil {
				logger.Warnw("failed to persist resources", zap.Error(err))

				// Retry
				p.sync()
			}
			atomic.StoreInt32(&p.syncing, 0)
		}
	}, 10*time.Millisecond)
}

// Sync triggers the synchronization process.
// If a sync is already taking place, it is interrupted before triggering it.
func (p *persistedStore) sync() {
	// Interrupt current sync process (if ongoing)
	atomic.StoreInt32(&p.syncing, 0)

	if len(p.syncCh) == 0 {
		p.syncCh <- true
	}
}

func (p *persistedStore) doSync(stopCh <-chan struct{}) error {
	cm, err := p.load()
	if err != nil {
		return err
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	// TODO: add support for partitioning

	config := make(map[string]interface{})
	objs := p.store.List()
	for _, obj := range objs {
		if len(stopCh) > 0 || atomic.LoadInt32(&p.syncing) == 0 {
			// either cancelled or interrupted.
			return nil
		}

		// Only add object that are ready
		kr := obj.(duckv1.KRShaped)
		if !kr.GetStatus().GetCondition(kr.GetConditionSet().GetTopLevelConditionType()).IsTrue() {
			continue
		}

		key := kr.GetNamespace() + "/" + kr.GetName()
		if projected := p.project(obj); projected != nil {
			config[key] = projected
		} else {
			delete(config, key)
		}
	}

	bconfig, err := json.Marshal(config)
	if err != nil {
		return err
	}
	newconfig := string(bconfig)

	if oldconfig, ok := cm.Data[ResourcesKey]; !ok || oldconfig != newconfig {
		cm.Data[ResourcesKey] = newconfig
		_, err = p.kubeClient.CoreV1().ConfigMaps(p.namespace).Update(cm)

		if err != nil {
			return err
		}
	}
	return nil
}

func (p *persistedStore) load() (*corev1.ConfigMap, error) {
	cm, err := p.kubeClient.CoreV1().ConfigMaps(p.namespace).Get(p.name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      p.name,
				Namespace: p.namespace,
				Labels: map[string]string{
					ComponentLabelKey: p.component,
				},
			},
			Data: map[string]string{},
		}

		return p.kubeClient.CoreV1().ConfigMaps(p.namespace).Create(cm)
	}

	return cm, nil
}

func (p *persistedStore) OnAdd(interface{}) {
	p.sync()
}

func (p *persistedStore) OnUpdate(interface{}, interface{}) {
	p.sync()
}

func (p *persistedStore) OnDelete(interface{}) {
	p.sync()
}
