/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package eventshub

import (
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/authentication/v1"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	// EventReason is the Kubernetes event reason used for observed events.
	CloudEventObservedReason = "CloudEventObserved"
)

type EventKind string

const (
	EventReceived EventKind = "Received"
	EventRejected EventKind = "Rejected"

	EventSent     EventKind = "Sent"
	EventResponse EventKind = "Response"

	PeerCertificatesReceived EventKind = "PeerCertificatesReceived"
)

type ConnectionTLS struct {
	CipherSuite           uint16   `json:"cipherSuite,omitempty"`
	CipherSuiteName       string   `json:"cipherSuiteName,omitempty"`
	HandshakeComplete     bool     `json:"handshakeComplete,omitempty"`
	IsInsecureCipherSuite bool     `json:"isInsecureCipherSuite,omitempty"`
	PemPeerCertificates   []string `json:"pemPeerCertificates,omitempty"`
}

type Connection struct {
	TLS *ConnectionTLS `json:"TLS,omitempty"`
}

// Structure to hold information about an event seen by eventshub pod.
type EventInfo struct {
	Kind EventKind `json:"kind"`

	// Set if the http request received by the pod couldn't be decoded or
	// didn't pass validation
	Error string `json:"error,omitempty"`
	// Event received if the cloudevent received by the pod passed validation
	Event *cloudevents.Event `json:"event,omitempty"`
	// In case there is a valid event in this instance, this contains all the HTTP headers,
	// including the CE- headers.
	HTTPHeaders map[string][]string `json:"httpHeaders,omitempty"`
	// In case there is a valid event in this instance, this field is not filled
	Body []byte `json:"body,omitempty"`

	StatusCode int `json:"statusCode,omitempty"`

	// Connection holds some underlying connection info like TLS, etc.
	Connection *Connection `json:"connection,omitempty"`

	Origin   string    `json:"origin,omitempty"`
	Observer string    `json:"observer,omitempty"`
	Time     time.Time `json:"time,omitempty"`
	Sequence uint64    `json:"sequence"`
	// SentId is just a correlator to correlate EventSent and EventResponse kinds.
	// This is filled with the ID of the sent event (if any) and in the Response also
	// jot it down so you can correlate which event (ID) as well as sequence to match sent/response 1:1.
	SentId string `json:"id"`

	// AdditionalInfo can be used by event generator implementations to add more event details
	AdditionalInfo map[string]interface{} `json:"additionalInfo"`

	// OIDCUserInfo is the user info of the subject of the OIDC token used in the request
	OIDCUserInfo *v1.UserInfo `json:"oidcUserInfo,omitempty"`
}

// Pretty print the event. Meant for debugging.
func (ei *EventInfo) String() string {
	var sb strings.Builder
	sb.WriteString("-- EventInfo --\n")
	sb.WriteString(fmt.Sprintf("--- Kind: %v ---\n", ei.Kind))
	if ei.Event != nil {
		sb.WriteString("--- Event ---\n")
		sb.WriteString(ei.Event.String())
		sb.WriteRune('\n')
	}
	if ei.Error != "" {
		sb.WriteString("--- Error ---\n")
		sb.WriteString(ei.Error)
		sb.WriteRune('\n')
	}
	if len(ei.HTTPHeaders) != 0 {
		sb.WriteString("--- HTTP headers ---\n")
		for k, v := range ei.HTTPHeaders {
			sb.WriteString("  " + k + ": " + v[0] + "\n")
		}
		sb.WriteRune('\n')
	}
	if ei.Body != nil {
		sb.WriteString("--- Body ---\n")
		sb.Write(ei.Body)
		sb.WriteRune('\n')
	}
	if ei.StatusCode != 0 {
		sb.WriteString(fmt.Sprintf("--- Status Code: %d ---\n", ei.StatusCode))
	}
	if ei.Connection != nil {
		sb.WriteString("--- Connection ---\n")
		c, _ := json.MarshalIndent(ei.Connection, "", "  ")
		sb.WriteString(string(c) + "\n")
	}
	sb.WriteString("--- Origin: '" + ei.Origin + "' ---\n")
	sb.WriteString("--- Observer: '" + ei.Observer + "' ---\n")
	sb.WriteString("--- Time: " + ei.Time.String() + " ---\n")
	sb.WriteString(fmt.Sprintf("--- Sequence: %d ---\n", ei.Sequence))
	sb.WriteString("--- Sent Id:  '" + ei.SentId + " ---\n")
	sb.WriteString("--------------------\n")
	return sb.String()
}

// This is mainly used for providing better failure messages
type SearchedInfo struct {
	TotalEvent int
	LastNEvent []EventInfo

	StoreEventsSeen    int
	StoreEventsNotMine int
}

// Pretty print the SearchedInfor for error messages
func (s *SearchedInfo) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d events seen, last %d events (total events seen %d, events ignored %d):\n",
		s.TotalEvent, len(s.LastNEvent), s.StoreEventsSeen, s.StoreEventsNotMine))
	for _, ei := range s.LastNEvent {
		sb.WriteString(ei.String())
		sb.WriteRune('\n')
	}
	return sb.String()
}

func TLSConnectionStateToConnection(state *tls.ConnectionState) *Connection {

	if state != nil {
		c := &Connection{TLS: &ConnectionTLS{}}
		c.TLS.CipherSuite = state.CipherSuite
		c.TLS.CipherSuiteName = tls.CipherSuiteName(state.CipherSuite)
		c.TLS.HandshakeComplete = state.HandshakeComplete
		c.TLS.IsInsecureCipherSuite = IsInsecureCipherSuite(state)

		for _, cert := range state.PeerCertificates {
			pemCert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
			c.TLS.PemPeerCertificates = append(c.TLS.PemPeerCertificates, pemCert)
		}

		return c
	}

	return nil
}

func IsInsecureCipherSuite(conn *tls.ConnectionState) bool {
	if conn == nil {
		return true
	}

	res := false
	for _, s := range tls.InsecureCipherSuites() {
		if s.ID == conn.CipherSuite {
			res = true
		}
	}
	return res
}
