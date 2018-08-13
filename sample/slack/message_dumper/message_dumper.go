/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"net/http"
	"net/http/httputil"
	"log"
)

type MessageDumper struct {}

func (md *MessageDumper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if reqBytes, err := httputil.DumpRequest(r, true); err == nil {
			log.Printf("Message Dumper received a message: %+v", string(reqBytes))
			w.Write(reqBytes)
		} else {
			log.Printf("Error dumping the request: %+v :: %+v", err, r)
		}
	}

func main() {
	http.ListenAndServe(":8080", &MessageDumper{})
}
