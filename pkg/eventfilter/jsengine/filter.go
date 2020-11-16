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

package jsengine

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/dop251/goja"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/eventfilter"
)

type jsFilter struct {
	engine *goja.Runtime
}

func (j *jsFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	pass, err := runFilter(event, j.engine)
	if err != nil {
		logging.FromContext(ctx).Warn("Error while trying to run the js expression filter: ", err)
		return eventfilter.FailFilter
	}
	if pass {
		return eventfilter.PassFilter
	} else {
		return eventfilter.FailFilter
	}
}

func NewJsFilter(src string) (eventfilter.Filter, error) {
	p, err := ParseFilterExpr(src)
	if err != nil {
		return nil, err
	}
	engine := goja.New()
	if _, err := runProgramWithSafeTimeout(timeout, engine, p); err != nil {
		return nil, err
	}
	return &jsFilter{
		engine: engine,
	}, nil
}
