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
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

func handlePost(rw http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	log.Println(string(body))

	var event map[string]string
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Failed to unmarshal event: %s", err)
		return
	}
	data, _ := base64.StdEncoding.DecodeString(event["Data"])
	log.Printf("Received data: %q", data)
}

func main() {
	http.HandleFunc("/", handlePost)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
