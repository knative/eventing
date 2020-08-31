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

package writer_vent

import (
	"context"
	"encoding/json"
	"io"

	"knative.dev/eventing/pkg/test/observer"
)

func NewEventLog(ctx context.Context, out io.Writer) observer.EventLog {
	return &writer{out: out}
}

type writer struct {
	out io.Writer
}

var newline = []byte("\n")

func (w *writer) Vent(observed observer.Observed) error {
	b, err := json.Marshal(observed)
	if err != nil {
		return err
	}
	if _, err := w.out.Write(b); err != nil {
		return err
	}
	if _, err := w.out.Write(newline); err != nil {
		return err
	}

	return nil
}
