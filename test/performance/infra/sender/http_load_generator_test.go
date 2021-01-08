/*
Copyright 2019 The Knative Authors
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
package sender

import (
	"bytes"
	"testing"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func TestGenerateRandStringPayload(t *testing.T) {
	const sizeRandomPayload = 10

	generated := generateRandStringPayload(sizeRandomPayload)

	if len(generated) != sizeRandomPayload {
		t.Errorf("len(generateRandStringPayload(sizeRandomPayload)) = %v, want %v", len(generated), sizeRandomPayload)
	}

	if generated[0] != markLetter {
		t.Errorf("generateRandStringPayload(sizeRandomPayload)[0] = %v, want %v", generated[0], markLetter)
	}

	if generated[sizeRandomPayload-1] != markLetter {
		t.Errorf("generateRandStringPayload(sizeRandomPayload)[sizeRandomPayload - 1] = %v, want %v", generated[sizeRandomPayload-1], markLetter)
	}
}

func TestVegetaTargeter(t *testing.T) {
	const sizeRandomPayload = 100

	for _, fixedPayload := range []bool{false, true} {
		expectedEventType := "test.event.type"
		expectedEventSource := "my.event.source"
		cet := NewCloudEventsTargeter("https://foo/bar", sizeRandomPayload, expectedEventType, expectedEventSource, fixedPayload)
		targeter := cet.VegetaTargeter()

		target1 := vegeta.Target{}
		if err := targeter(&target1); err != nil {
			t.Fatal("Targeter call returned error:", err)
		}

		nonEmptyHeaders := []string{"Ce-Id", "Ce-Type", "Ce-Source", "Ce-Specversion", "Content-Type"}
		for _, header := range nonEmptyHeaders {
			val, found := target1.Header[header]
			if !found {
				t.Fatal("Missing header:", header)
			} else if len(val) != 1 {
				t.Fatalf("Bad header(%s) length = %d, expected 1", header, len(val))
			}
		}
		ceType := target1.Header["Ce-Type"]
		if ceType[0] != expectedEventType {
			t.Errorf("Unexpected event type = %s, want %s", ceType, expectedEventType)
		}
		ceSource := target1.Header["Ce-Source"]
		if ceSource[0] != "my.event.source" {
			t.Errorf("Unexpected event type = %s, want %s", ceSource, expectedEventSource)
		}
		target2 := vegeta.Target{}
		if err := targeter(&target2); err != nil {
			t.Fatal("Targeter call returned error:", err)
		}
		if fixedPayload && !bytes.Equal(target1.Body, target2.Body) {
			t.Errorf("Target bodies differ, b1 = %v, b2 = %v", target1.Body, target2.Body)
		} else if !fixedPayload && bytes.Equal(target1.Body, target2.Body) {
			t.Error("Target bodies unexpectedly equal:", target1.Body)
		}
		ceID1 := target1.Header["Ce-Id"]
		ceID2 := target2.Header["Ce-Id"]
		if len(ceID1) == 1 && len(ceID2) == 1 {
			if ceID1[0] == ceID2[0] {
				t.Errorf("unexpectedly matching message ID's: %s, %s", ceID1, ceID2)
			}
		} else {
			t.Errorf("bad header id list lengths %d, %d, expected 1", len(ceID1), len(ceID2))
		}
	}
}

func BenchmarkTargeterFixedPayload(b *testing.B) {
	const sizeRandomPayload = 100

	cet := NewCloudEventsTargeter("https://foo/bar", sizeRandomPayload, "test.event.type", "my.event.source", true)

	targeter := cet.VegetaTargeter()

	for i := 0; i < b.N; i++ {
		target1 := vegeta.Target{}
		if err := targeter(&target1); err != nil {
			b.Fatalf("Targeter call returned error: %v", err)
		}
	}
}

func BenchmarkTargeterRandomPayload(b *testing.B) {
	const sizeRandomPayload = 100

	cet := NewCloudEventsTargeter("https://foo/bar", sizeRandomPayload, "test.event.type", "my.event.source", false)

	targeter := cet.VegetaTargeter()

	for i := 0; i < b.N; i++ {
		target1 := vegeta.Target{}
		if err := targeter(&target1); err != nil {
			b.Fatalf("Targeter call returned error: %v", err)
		}
	}
}
