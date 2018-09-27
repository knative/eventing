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

// A sidecar that implements filtering of Cloud Events sent out via HTTP. Implemented as an HTTP
// proxy that the main containers need to write through.

package main

import (
	"flag"
	"fmt"
	"github.com/knative/eventing/pkg/sidecar/configmaphandler"
	"go.uber.org/zap"
	"net/http"
	"time"
)

const (
	port = 11235
)

var (
	readTimeout  = time.Minute
	writeTimeout = time.Minute
)

func main() {
	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	cmh, err := configmaphandler.NewHandler(logger, configmaphandler.ConfigDir)
	if err != nil {
		logger.Fatal("Unable to create configmaphandler.Handler", zap.Error(err))
	}
	s := &http.Server{
		Addr:         fmt.Sprintf(":%v", port),
		Handler:      cmh,
		ErrorLog:     zap.NewStdLog(logger),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
	logger.Info("Fanout sidecar Listening...")
	s.ListenAndServe()
}
