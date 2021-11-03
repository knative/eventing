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

package testing

import (
	"sync"

	"knative.dev/eventing/pkg/scheduler"
)

type VPodStore struct {
	vpods []scheduler.VPod
	lock  sync.Mutex
}

func newVPodStore() *VPodStore {
	return &VPodStore{}
}

func newVPodLister(store *VPodStore) scheduler.VPodLister {
	return func() ([]scheduler.VPod, error) {
		store.lock.Lock()
		defer store.lock.Unlock()
		dst := make([]scheduler.VPod, len(store.vpods))
		copy(dst, store.vpods)
		return dst, nil
	}
}
