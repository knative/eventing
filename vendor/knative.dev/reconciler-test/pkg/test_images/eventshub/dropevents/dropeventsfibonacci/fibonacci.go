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

package dropeventsfibonacci

type Fibonacci struct {
	Prev    uint64
	Current uint64
}

func (f *Fibonacci) Skip(counter uint64) bool {
	if f.Current == counter {
		f.Next()
		return true
	}
	return false
}

func (f *Fibonacci) Next() {
	f.Prev, f.Current = f.Current, f.Prev+f.Current
}
