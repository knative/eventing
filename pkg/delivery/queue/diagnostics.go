/*
Copyright 2018 Google, Inc. All rights reserved.

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

package queue

import (
	"encoding/json"
	"net/http"

	"github.com/golang/glog"
)

type diagnosticsHandler struct {
	Queue
	*http.ServeMux
}

// NewDiagnosticHandler provides endpoints for inspecting
// and diagnosing queued events.
// Eventually we'll need to decide more about how we will shard our queues
// and what visibility we'll give to each. E.g. I suspect we'll support looking
// at the length of queues for a single Flow or Action.
func NewDiagnosticHandler(q Queue) http.Handler {
	h := &diagnosticsHandler{Queue: q, ServeMux: http.NewServeMux()}
	h.HandleFunc("/queue/length", h.GetLength)
	h.HandleFunc("/queue/", func(w http.ResponseWriter, r *http.Request) {
		glog.Infof("Do not know how to handle queue path", r.URL.Path)
		http.NotFound(w, r)
	})
	return h
}

func (h *diagnosticsHandler) GetLength(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		glog.Warningf("Unauthorized method on /queue/length")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	glog.V(4).Info("Reporting queue length")
	b, err := json.Marshal(map[string]int{"length": h.Length()})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Write(b)
}
