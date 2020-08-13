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
	"fmt"
	"testing"
	"time"
)

func TestWithDelayedCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	delayedCtx := WithDelayedCancellation(ctx, 55)
	go func() {
		<-delayedCtx.Done()
	}()

	fmt.Println(time.Now())
	waitForSecond(56)
	cancel()

	<-delayedCtx.Done()

	fmt.Println(time.Now())
	csecond := time.Now().Second()
	if csecond > 55 {
		t.Error("expected current second to be less than 55")
	}
}

func waitForSecond(second int) {
	csecond := time.Now().Second()
	if csecond == second {
		return
	}
	if csecond > second {
		time.Sleep(time.Duration(second+60-csecond) * time.Second)
	} else {
		time.Sleep(time.Duration(second-csecond) * time.Second)
	}
}
