/*
 * Copyright 2018 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/go-martini/martini"
)

var (
	// TODO make thread safe
	subscriptions  = make(map[string]map[string]struct{})
	exists         = struct{}{}
	broker         = os.Getenv("BROKER_NAME")
	forwardHeaders = []string{
		"content-type",
		"x-request-id",
		"x-b3-traceid",
		"x-b3-spanid",
		"x-b3-parentspanid",
		"x-b3-sampled",
		"x-b3-flags",
		"x-ot-span-context",
	}
)

func splitStreamName(host string) string {
	chunks := strings.Split(host, ".")
	stream := chunks[0]
	return stream
}

func main() {
	m := martini.Classic()

	m.Post("/", func(req *http.Request, res http.ResponseWriter) {
		host := req.Host
		fmt.Printf("Recieved request for %s\n", host)
		stream := splitStreamName(host)
		subscribers, ok := subscriptions[stream]
		if !ok {
			res.WriteHeader(http.StatusNotFound)
			return
		}

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		res.WriteHeader(http.StatusAccepted)
		go func() {
			fmt.Printf("Making subscribed requests for %s\n", stream)

			// make upstream requests
			client := &http.Client{}

			for subscribed := range subscribers {
				go func(subscribed string) {
					fmt.Printf("Making subscribed request to %s for %s\n", subscribed, stream)

					url := fmt.Sprintf("http://%s/", subscribed)
					request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
					if err != nil {
						fmt.Printf("Unable to create subscriber request %v", err)
					}
					request.Header.Set("x-broker", broker)
					request.Header.Set("x-stream", stream)
					for _, header := range forwardHeaders {
						if value := req.Header.Get(header); value != "" {
							request.Header.Set(header, value)
						}
					}
					_, err = client.Do(request)
					if err != nil {
						fmt.Printf("Unable to complete subscriber request %v", err)
					}
				}(subscribed)
			}
		}()
	})

	m.Group("/streams/:stream", func(r martini.Router) {
		r.Put("", func(params martini.Params, res http.ResponseWriter) {
			stream := params["stream"]
			fmt.Printf("Create stream %s\n", stream)
			if _, ok := subscriptions[stream]; !ok {
				subscriptions[stream] = map[string]struct{}{}
			}
			res.WriteHeader(http.StatusAccepted)
		})
		r.Delete("", func(params martini.Params, res http.ResponseWriter) {
			stream := params["stream"]
			fmt.Printf("Delete stream %s\n", stream)
			delete(subscriptions, stream)
			res.WriteHeader(http.StatusAccepted)
		})

		r.Group("/subscriptions/:subscription", func(r martini.Router) {
			r.Put("", func(params martini.Params, res http.ResponseWriter) {
				stream := params["stream"]
				subscription := params["subscription"]
				subscribers, ok := subscriptions[stream]
				if !ok {
					res.WriteHeader(http.StatusNotFound)
					return
				}
				fmt.Printf("Create subscription %s for stream %s\n", subscription, stream)
				// TODO store subscription params
				subscribers[subscription] = exists
				res.WriteHeader(http.StatusAccepted)
			})
			r.Delete("", func(params martini.Params, res http.ResponseWriter) {
				stream := params["stream"]
				subscription := params["subscription"]
				subscribers, ok := subscriptions[stream]
				if !ok {
					res.WriteHeader(http.StatusNotFound)
					return
				}
				fmt.Printf("Delete subscription %s for stream %s\n", subscription, stream)
				delete(subscribers, subscription)
				res.WriteHeader(http.StatusAccepted)
			})
		})
	})

	m.Run()
}
