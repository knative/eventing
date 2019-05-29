package configmap

import (
	"github.com/knative/pkg/configmap"
	v1 "k8s.io/api/core/v1"
)

// TODO Move this to knative/pkg.

// DefaultConstructors are something.
type DefaultConstructors map[*v1.ConfigMap]interface{}

// DefaultUntypedStore is an UntypedStore with default values for ConfigMaps that do not exist.
type DefaultUntypedStore struct {
	store      *configmap.UntypedStore
	defaultCMs []v1.ConfigMap
}

// NewDefaultUntypedStore creates a new DefaultUntypedStore.
func NewDefaultUntypedStore(
	name string,
	logger configmap.Logger,
	defaultConstructors DefaultConstructors,
	onAfterStore ...func(name string, value interface{})) *DefaultUntypedStore {
	constructors := configmap.Constructors{}
	defaultCMs := make([]v1.ConfigMap, 0, len(defaultConstructors))
	for cm, ctor := range defaultConstructors {
		constructors[cm.Name] = ctor
		defaultCMs = append(defaultCMs, *cm)
	}
	return &DefaultUntypedStore{
		store:      configmap.NewUntypedStore(name, logger, constructors, onAfterStore...),
		defaultCMs: defaultCMs,
	}
}

// WatchConfigs uses the provided configmap.DefaultingWatcher to setup watches for the config maps
// provided in defaultCMs.
func (s *DefaultUntypedStore) WatchConfigs(w configmap.DefaultingWatcher) {
	for _, cm := range s.defaultCMs {
		w.WatchWithDefault(cm, s.store.OnConfigChanged)
	}
}
