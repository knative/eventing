/*
Copyright 2021 The Knative Authors

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

package feature

import "time"

// T is an interface similar to testing.T passed to StepFn to perform logging and assertions
type T interface {
	Name() string

	Log(args ...interface{})
	Logf(format string, args ...interface{})

	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fail()

	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	FailNow()

	Skip(args ...interface{})
	Skipf(format string, args ...interface{})
	SkipNow()

	Failed() bool
	Skipped() bool

	// Note: these are step scoped!
	Cleanup(f func())
	Deadline() (deadline time.Time, ok bool)
}
