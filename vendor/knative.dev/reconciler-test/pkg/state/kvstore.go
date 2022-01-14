/*
Copyright 2021 The Knative Authors

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

package state

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type KVStore struct {
	store map[string]string
	mux   sync.Mutex
}

// Get retrieves and unmarshals the value from the map.
func (cs *KVStore) Get(_ context.Context, key string, value interface{}) error {
	cs.mux.Lock()
	defer cs.mux.Unlock()

	if cs.store == nil {
		cs.store = make(map[string]string)
	}

	v, ok := cs.store[key]

	if !ok {
		return fmt.Errorf("key %s does not exist", key)
	}
	err := json.Unmarshal([]byte(v), value)
	if err != nil {
		return fmt.Errorf("failed to Unmarshal %q: %v", v, err)
	}
	return nil
}

// Set marshals and sets the value given under specified key.
func (cs *KVStore) Set(_ context.Context, key string, value interface{}) error {
	cs.mux.Lock()
	defer cs.mux.Unlock()

	if cs.store == nil {
		cs.store = make(map[string]string)
	}

	bytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to Marshal: %v", err)
	}
	cs.store[key] = string(bytes)
	return nil
}

func (cs *KVStore) MarshalJSON() ([]byte, error) {
	return json.Marshal(cs.store)
}
