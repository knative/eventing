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

package clientfactory

import "net/http"

// ClientFactory creates HTTP clients.
type ClientFactory interface {
	Create() (HttpDoer, error)
}

// ClientFactory that creates http.Client.
type Standard struct{}

func (_ *Standard) Create() (HttpDoer, error) {
	return &http.Client{}, nil
}

// Makes generic HTTP requests.
type HttpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}
