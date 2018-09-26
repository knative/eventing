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

package fake

import (
	"errors"
	"github.com/knative/eventing/pkg/sidecar/clientfactory"
	"net/http"
	"sync"
)

// A fake ClientFactory that will return the errors or responses specified. As well as record all
// requests made.
type ClientFactory struct {
	// Error to return when calling Create().
	CreateErr error
	// Guards req.
	reqMu sync.Mutex
	// The HTTP requests made. Only access when holding reqMu.
	req []*http.Request

	// Error to return when calling Do().
	RespErr error

	// Resp are the responses to send out. All entries are sent once, except for the last response,
	// which is sent on all subsequent calls to Do().
	Resp []*http.Response
}

func (f *ClientFactory) Create() (clientfactory.HttpDoer, error) {
	if f.CreateErr != nil {
		return nil, f.CreateErr
	}
	return f, nil
}

func (f *ClientFactory) Do(r *http.Request) (*http.Response, error) {
	f.reqMu.Lock()
	defer f.reqMu.Unlock()
	f.req = append(f.req, r)
	if f.RespErr != nil {
		return nil, f.RespErr
	}
	if len(f.Resp) == 0 {
		return nil, errors.New("test failure -- no response in fake.ClientFactory")
	}
	resp := f.Resp[0]
	if len(f.Resp) > 1 {
		f.Resp = f.Resp[1:]
	}
	return resp, nil
}

// GetRequests return all the requests made so far.
func (f *ClientFactory) GetRequests() []*http.Request {
	f.reqMu.Lock()
	defer f.reqMu.Unlock()
	rv := make([]*http.Request, len(f.req))
	copy(rv, f.req)
	return rv
}
