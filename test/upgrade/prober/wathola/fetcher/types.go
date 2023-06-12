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

package fetcher

import (
	"os"
	"time"

	"knative.dev/eventing/test/upgrade/prober/wathola/receiver"
)

// Fetcher will fetch a report from remote receiver service.
type Fetcher interface {
	FetchReport()
}

// Execution represents a response from remote receiver service and local logs
// that were received.
type Execution struct {
	Logs   []LogEntry       `json:"logs"`
	Report *receiver.Report `json:"report"`
}

// LogEntry represents a single
type LogEntry struct {
	Level    string    `json:"level"`
	Datetime time.Time `json:"dt"`
	Message  string    `json:"msg"`
}

type fetcher struct {
	out *os.File
	log *memoryLogger
}

type memoryLogger struct {
	*Execution
}
