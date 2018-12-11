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
	"k8s.io/api/core/v1"
	"testing"
)

func TestMessageHistoryEmpty(t *testing.T) {
	m := Message{}
	history := getHistoryOrFail(t, &m)
	if len(history) != 0 {
		t.Error("history not empty")
	}
}

func TestMessageHistoryAppendSet(t *testing.T) {
	m := Message{}
	m.AppendToHistory(ChannelReference{
		Namespace: "namespace1",
		Name:      "name1",
	}, true)
	history := getHistoryOrFail(t, &m)
	if len(history) != 1 {
		t.Error("history does not contain first element")
	}
	m.AppendToHistory(ChannelReference{
		Namespace: "namespace2",
		Name:      "name2",
	}, true)
	history = getHistoryOrFail(t, &m)
	if len(history) != 2 {
		t.Error("history does not contain all elements")
	}
	if history[0].Name != "name1" {
		t.Error("wrong name")
	}
	if history[0].Namespace != "namespace1" {
		t.Error("wrong namespace")
	}
	if history[1].Name != "name2" {
		t.Error("wrong name")
	}
	if history[1].Namespace != "namespace2" {
		t.Error("wrong namespace")
	}
	newHistory := []v1.ObjectReference{
		{
			Namespace: "namespace3",
			Name:      "name3",
		},
	}
	m.SetHistory(newHistory)
	history = getHistoryOrFail(t, &m)
	if len(history) != 1 {
		t.Error("history does not contain the new element")
	}
	if history[0] != newHistory[0] {
		t.Error("wrong history element")
	}
}

func TestMessageHistoryAppendOverwrite(t *testing.T) {
	m := Message{}
	m.Headers = make(map[string]string)
	m.Headers[messageHistoryHeader] = "-unparsable-"
	if _, err := m.History(); err == nil {
		t.Error("read error not thrown")
	}
	ch := ChannelReference{
		Name: "err",
	}
	if err := m.AppendToHistory(ch, false); err == nil {
		t.Error("append error not thrown")
	}
	ch.Name = "name"
	if err := m.AppendToHistory(ch, true); err != nil {
		t.Error("unexpected error thrown")
	}
	history := getHistoryOrFail(t, &m)
	if len(history) != 1 || history[0].Name != "name" {
		t.Error("wrong history")
	}
}

func getHistoryOrFail(t *testing.T, m *Message) []v1.ObjectReference {
	history, err := m.History()
	if err != nil {
		t.Errorf("got %v when getting history", err)
	}
	return history
}
