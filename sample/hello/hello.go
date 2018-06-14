/*
Copyright 2018 Google LLC

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
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func handlePost(rw http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	greeting := fmt.Sprintf("Hello %s, from %s", string(body), hostname)
	log.Println(greeting)

	rw.WriteHeader(200)
	rw.Header().Add("content-type", "text/plain")
	rw.Write([]byte(greeting))
}

func main() {
	http.HandleFunc("/", handlePost)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
