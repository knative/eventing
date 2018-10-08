/*
Copyright 2018 The Knative Authors
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

// cleanup allows you to define a cleanup function that will be executed
// if your test is interrupted.

package test

import (
	"os"
	"os/signal"

	"github.com/knative/pkg/test/logging"
)

// Cleaner holds resources that will be cleaned after test
type Cleaner struct {
	resourcesToClean []ResourceDeleter
	logger           *logging.BaseLogger
	clients          *Clients
}

// NewCleaner creates a new Cleaner
func NewCleaner(log *logging.BaseLogger, testClients *Clients) *Cleaner {
	cleaner := &Cleaner{
		resourcesToClean: make([]ResourceDeleter, 0, 10),
		logger:           log,
		clients:          testClients,
	}
	return cleaner
}

// Add will register a resources to be cleaned by the Clean function
func (cleaner *Cleaner) Add(resource interface{}, name string) error {
	res := ResourceDeleter{
		Resource: resource,
		Name:     name,
	}
	//this is actually a prepend, we want to delete resources in reverse order
	cleaner.resourcesToClean = append([]ResourceDeleter{res}, cleaner.resourcesToClean...)
	return nil
}

// Clean will delete all registered resources
func (cleaner *Cleaner) Clean() error {
	for _, resource := range cleaner.resourcesToClean {
		cleaner.clients.Delete(resource.Resource, resource.Name)
	}
	return nil
}

// ResourceDeleter holds the cleaner and name of resource to be cleaned
type ResourceDeleter struct {
	Resource interface{}
	Name     string
}

// CleanupOnInterrupt will execute the function cleanup if an interrupt signal is caught
func CleanupOnInterrupt(cleanup func(), logger *logging.BaseLogger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			logger.Infof("Test interrupted, cleaning up.")
			cleanup()
			os.Exit(1)
		}
	}()
}
