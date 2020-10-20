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

package recordevents

// EventLog is the contract for an event logger to vent an event.
type EventLog interface {
	Vent(observed EventInfo) error
}

type EventLogs []EventLog

func (e EventLogs) Vent(observed EventInfo) error {
	for _, el := range e {
		if err := el.Vent(observed); err != nil {
			return err
		}
	}
	return nil
}
