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

package lib

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/wait"
)

// WaitFor will register a wait routine to be resolved later
func WaitFor(name string, routine AwaitRoutine) {
	w := &namedAwait{
		name:    name,
		routine: routine,
	}
	waits = append(waits, w)
}

// AwaitForAll will wait until all registered wait routines resolves
func AwaitForAll(log *zap.SugaredLogger) error {
	var g errgroup.Group
	for _, w := range waits {
		w := w // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			log.Infof("Wait for %s", w.name)
			before := time.Now()
			err := w.routine()
			took := time.Since(before)
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
		return err
	}
	return nil
}

// WaitForReadiness will wait until readiness endpoint reports OK
func WaitForReadiness(port int, log *zap.SugaredLogger) error {
	return wait.PollImmediate(10*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", port))
		if err != nil {
			log.Debugf("Error while connecting: %v", err)
			return false, nil
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, err
		}
		return resp.StatusCode == 200 && string(body) == "OK", nil
	})
}

type AwaitRoutine func() error

type namedAwait struct {
	name    string
	routine AwaitRoutine
}

var waits []*namedAwait
