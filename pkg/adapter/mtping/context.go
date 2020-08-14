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

package mtping

import (
	"context"
	"time"
)

// NewDelayingContext returns a new context delaying the
// cancellation of the given context.
func NewDelayingContext(ctx context.Context, afterSecond int) context.Context {
	delayCtx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()

		s := time.Now().Second()
		if s > afterSecond {
			// Make sure to pass the minute by adding 1 second.
			time.Sleep(time.Second * time.Duration(60-s+1))
		}

		cancel()
	}()
	return delayCtx
}
