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
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
)

func (p *prober) waitForTriggerReady(name, namespace string) error {
	meta := resources.NewMetaResource(name, namespace, lib.TriggerTypeMeta)
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
