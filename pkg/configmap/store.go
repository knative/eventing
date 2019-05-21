package configmap

import (
	"github.com/knative/pkg/configmap"
	v1 "k8s.io/api/core/v1"
)

type DefaultConstructors map[v1.ConfigMap]interface{}

type DefaultUntypedStore struct {
	store      *configmap.UntypedStore
	defaultCMs []v1.ConfigMap
}

func NewDefaultUntypedStore(
	name string,
	logger configmap.Logger,
	defaultConstructors DefaultConstructors,
	onAfterStore ...func(name string, value interface{})) *DefaultUntypedStore {
	constructors := configmap.Constructors{}
	defaultCMs := make([]v1.ConfigMap, 0, len(defaultConstructors))
	for cm, ctor := range defaultConstructors {
		constructors[cm.Name] = ctor
		defaultCMs = append(defaultCMs, cm)
	}
	return &DefaultUntypedStore{
		store:      configmap.NewUntypedStore(name, logger, constructors, onAfterStore...),
		defaultCMs: defaultCMs,
	}
}

func (s *DefaultUntypedStore) WatchConfigs(w configmap.DefaultingWatcher) {
	for _, cm := range s.defaultCMs {
		w.WatchWithDefault(cm, s.store.OnConfigChanged)
	}
}
