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

package milestone

import (
	"knative.dev/reconciler-test/pkg/feature"
)

// Compose creates an Emitter from a list of Emitters.
func Compose(emitters ...Emitter) Emitter {
	return compositeEmitter{emitters: emitters}
}

type compositeEmitter struct {
	emitters []Emitter
}

func (c compositeEmitter) Environment(env map[string]string) {
	c.foreach(func(emitter Emitter) { emitter.Environment(env) })
}

func (c compositeEmitter) NamespaceCreated(namespace string) {
	c.foreach(func(emitter Emitter) { emitter.NamespaceCreated(namespace) })
}

func (c compositeEmitter) NamespaceDeleted(namespace string) {
	c.foreach(func(emitter Emitter) { emitter.NamespaceDeleted(namespace) })
}

func (c compositeEmitter) TestStarted(feature string, t feature.T) {
	c.foreach(func(emitter Emitter) { emitter.TestStarted(feature, t) })
}

func (c compositeEmitter) TestFinished(feature string, t feature.T) {
	c.foreach(func(emitter Emitter) { emitter.TestFinished(feature, t) })
}

func (c compositeEmitter) StepsPlanned(feature string, steps map[feature.Timing][]feature.Step, t feature.T) {
	c.foreach(func(emitter Emitter) { emitter.StepsPlanned(feature, steps, t) })
}

func (c compositeEmitter) StepStarted(feature string, step *feature.Step, t feature.T) {
	c.foreach(func(emitter Emitter) { emitter.StepStarted(feature, step, t) })
}

func (c compositeEmitter) StepFinished(feature string, step *feature.Step, t feature.T) {
	c.foreach(func(emitter Emitter) { emitter.StepFinished(feature, step, t) })
}

func (c compositeEmitter) TestSetStarted(featureSet string, t feature.T) {
	c.foreach(func(emitter Emitter) { emitter.TestSetStarted(featureSet, t) })
}

func (c compositeEmitter) TestSetFinished(featureSet string, t feature.T) {
	c.foreach(func(emitter Emitter) { emitter.TestSetFinished(featureSet, t) })
}

func (c compositeEmitter) Finished() {
	c.foreach(func(emitter Emitter) { emitter.Finished() })
}

func (c compositeEmitter) Exception(reason, messageFormat string, messageA ...interface{}) {
	c.foreach(func(emitter Emitter) { emitter.Exception(reason, messageFormat, messageA...) })
}

func (c compositeEmitter) foreach(f func(emitter Emitter)) {
	for _, e := range c.emitters {
		f(e)
	}
}
