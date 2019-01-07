/*
Copyright 2018 The Knative Authors

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

package provisioners

import (
	"testing"
)

func TestMessageHistoryEmpty(t *testing.T) {
	m := Message{}
	history := m.History()
	if len(history) != 0 {
		t.Error("history not empty")
	}
}

func TestMessageHistoryAppendSet(t *testing.T) {
	m := Message{}
	m.AppendToHistory("name1.ns1.service.local")
	history := m.History()
	if len(history) != 1 {
		t.Error("history does not contain first element")
	}
	m.AppendToHistory("name2.ns2.service.local")
	history = m.History()
	if len(history) != 2 {
		t.Error("history does not contain all elements")
	}
	if history[0] != "name1.ns1.service.local" {
		t.Error("wrong name")
	}
	if history[1] != "name2.ns2.service.local" {
		t.Error("wrong name")
	}
	newHistory := []string{"name3.ns3.service.local"}
	m.SetHistory(newHistory)
	history = m.History()
	if len(history) != 1 {
		t.Error("history does not contain the new element")
	}
	if history[0] != newHistory[0] {
		t.Error("wrong history element")
	}
	m.AppendToHistory("")
	m.AppendToHistory(" ")
	m.AppendToHistory("  ")
	history = m.History()
	if len(history) != 1 {
		t.Error("history contains unexpected elements")
	}
}
