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

package event_test

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/knative/eventing/pkg/event"
)

type FirestoreDocument struct {
	Name       string                 `json:"name"`
	Fields     map[string]interface{} `json:"fields"`
	CreateTime time.Time              `json:"createTime"`
	UpdateTime time.Time              `json:"updateTime"`
}

var (
	webhook = "http://localhost/:sendEvent"
)

// Wrap the global functions in an HTTPMarshaller interface for table-driven testing:
type defaultMarshaller int

var Default defaultMarshaller = 0

func (defaultMarshaller) FromRequest(data interface{}, r *http.Request) (*event.EventContext, error) {
	return event.FromRequest(data, r)
}
func (defaultMarshaller) NewRequest(urlString string, data interface{}, context event.EventContext) (*http.Request, error) {
	return event.NewRequest(urlString, data, context)
}

func TestValidRoundTrips(t *testing.T) {
	doc := FirestoreDocument{
		Name: "projects/demo/databases/default/documents/users/inlined",
		Fields: map[string]interface{}{
			"project": "eventing",
			"handle":  "@inlined",
		},
		CreateTime: time.Date(1985, 6, 5, 12, 0, 0, 0, time.UTC),
		UpdateTime: time.Now().UTC(),
	}

	service := "firestore.googleapis.com"

	context := &event.EventContext{
		CloudEventsVersion: "0.1",
		EventID:            "eventid-123",
		EventTime:          doc.UpdateTime,
		EventType:          "google.firestore.document.create",
		EventTypeVersion:   "v1beta2",
		SchemaURL:          "http://type.googleapis.com/google.firestore.v1beta1.Document",
		ContentType:        "application/json",
		Source:             fmt.Sprintf("//%s/%s", service, doc.Name),
		Extensions: map[string]interface{}{
			"purpose": "tbd",
		},
	}
	for _, test := range []struct {
		name    string
		encoder event.HTTPMarshaller
		decoder event.HTTPMarshaller
	}{
		{
			name:    "binary -> binary",
			encoder: event.Binary,
			decoder: event.Binary,
		},
		{
			name:    "binary -> default",
			encoder: event.Binary,
			decoder: Default,
		},
		{
			name:    "structured -> structured",
			encoder: event.Structured,
			decoder: event.Structured,
		},
		{
			name:    "structured -> default",
			encoder: event.Structured,
			decoder: Default,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			req, err := test.encoder.NewRequest(webhook, doc, *context)
			if err != nil {
				t.Fatalf("Failed to encode event %s", err)
			}

			var foundData FirestoreDocument
			foundContext, err := test.decoder.FromRequest(&foundData, req)
			if err != nil {
				t.Fatalf("Failed to decode event %s", err)
			}

			if !reflect.DeepEqual(context, foundContext) {
				t.Fatalf("Context was transcoded lossily: expected=%+v got=%+v", context, foundContext)
			}
			if !reflect.DeepEqual(doc, foundData) {
				t.Fatalf("Data was transcoded lossily: expected=%+v got=%+v", doc, foundData)
			}
		})
	}
}

type Address struct {
	City, State string
}
type Person struct {
	XMLName   xml.Name `xml:"person"`
	Id        int      `xml:"id,attr"`
	FirstName string   `xml:"name>first"`
	LastName  string   `xml:"name>last"`
	Age       int      `xml:"age"`
	Height    float32  `xml:"height,omitempty"`
	Married   bool
	Address
	Comment string `xml:",comment"`
}

func (Person) MarshalJSON() ([]byte, error) {
	return nil, errors.New("Person cannot be JSON encoded")
}

func (*Person) UnmarshalJSON([]byte) error {
	return errors.New("Person cannot be JSON decoded")
}

func TestXmlStructuredDecoding(t *testing.T) {
	person := &Person{
		XMLName: xml.Name{
			Local: "person",
		},
		Id:        13,
		FirstName: "John",
		LastName:  "Doe",
		Age:       42,
		Comment:   " Need more details. ",
		Address:   Address{"Hanga Roa", "Easter Island"},
	}

	xmlPerson := `
		<person id="13">
			<name>
				<first>John</first>
				<last>Doe</last>
			</name>
			<age>42</age>
			<Married>false</Married>
			<City>Hanga Roa</City>
			<State>Easter Island</State>
			<!-- Need more details. -->
		</person>`
	xmlJsonSafe, err := json.Marshal(xmlPerson)
	if err != nil {
		t.Fatalf("Failed to create JSON encoded XML string: %s", err)
	}

	eventText := `
	{
		"eventID": "1234",
		"eventType": "dev.eventing.test",
		"source": "tests://TextXmlStructuredEncoding",
		"contentType": "application/xml",
		"data": ` + string(xmlJsonSafe) + `
	}`

	h := http.Header{}
	h.Set(event.HeaderContentType, event.ContentTypeStructuredJSON)
	req := &http.Request{
		Header: h,
		Body:   ioutil.NopCloser(strings.NewReader(eventText)),
	}

	var foundPerson Person
	_, err = event.FromRequest(&foundPerson, req)
	if err != nil {
		t.Fatalf("Failed to parse cross-encoded request: %s", err)
	}

	if !reflect.DeepEqual(person, &foundPerson) {
		t.Fatalf("Failed to parse xml-encoded data; wanted=%+v; got=%+v", person, foundPerson)
	}
}

func TestExtensionsAreNeverNil(t *testing.T) {
	r := &http.Request{
		Header: http.Header{
			event.HeaderContentType: []string{event.ContentTypeStructuredJSON},
		},
		Body: ioutil.NopCloser(strings.NewReader(`
			{
				"eventID": "1234",
				"eventType": "dev.eventing.test",
				"source": "tests://TextXmlStructuredEncoding",
				"data": "hello, world"
			}`)),
	}

	var data interface{}
	ctx, err := event.FromRequest(&data, r)
	if err != nil {
		t.Fatal("Failed to parse request", err)
	}
	if ctx.Extensions == nil {
		t.Fatal("Extensions should never be nil")
	}
}

func TestExtensionExtraction(t *testing.T) {
	h := http.Header{}
	h.Set(event.HeaderContentType, event.ContentTypeBinaryJSON)
	h.Set(event.HeaderEventID, "1234")
	h.Set(event.HeaderEventType, "dev.eventing.test")
	h.Set(event.HeaderSource, "tests://TestExtensionExtraction")
	h.Set("CE-X-Prop1", "value1")
	h.Set("CE-X-Prop2", `{"nestedProp":"nestedValue"}`)
	b := strings.NewReader(`{"hello": "world"}`)

	r := &http.Request{
		Header: h,
		Body:   ioutil.NopCloser(b),
	}
	var data interface{}
	ctx, err := event.FromRequest(&data, r)
	if err != nil {
		t.Fatal("Failed to parse request", err)
	}

	expectedExtensions := map[string]interface{}{
		"Prop1": "value1",
		"Prop2": map[string]interface{}{
			"nestedProp": "nestedValue",
		},
	}
	if !reflect.DeepEqual(expectedExtensions, ctx.Extensions) {
		t.Fatalf("Did not parse expected extensions. Wanted=%v; got=%v", expectedExtensions, ctx.Extensions)
	}
}
