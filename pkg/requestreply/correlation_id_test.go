/*
Copyright 2025 The Knative Authors

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

package requestreply

import (
	"fmt"
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/assert"
)

var (
	exampleKey = []byte("passphrasewhichneedstobe32bytes!")
	otherKey   = []byte("passphrasewhichneedstobe32bytes?")
	badKey     = []byte("passphrasewhichneedstobe32bytesbutistoolong")
)

func TestCorrelationIdVerification(t *testing.T) {
	tests := map[string]struct {
		replyIdName           string
		correlationIdName     string
		encryptionKey         []byte
		decryptionKey         []byte
		transformEvent        func(ce *cloudevents.Event)
		expectEncryptionError bool
		expectDecryptionError bool
		expectValid           bool
	}{
		"matching encryption and decryption keys": {
			encryptionKey: exampleKey,
			decryptionKey: exampleKey,
			transformEvent: func(ce *cloudevents.Event) {
				ce.SetExtension("replyid", ce.Extensions()["correlationid"])
			},
			expectValid: true,
		},
		"mismatched encryption and decryption keys": {
			encryptionKey: exampleKey,
			decryptionKey: otherKey,
			transformEvent: func(ce *cloudevents.Event) {
				ce.SetExtension("replyid", ce.Extensions()["correlationid"])
			},
			expectDecryptionError: true,
		},
		"invalid encryption key": {
			encryptionKey:         badKey,
			expectEncryptionError: true,
		},
		"invalid replyid": {
			encryptionKey: exampleKey,
			decryptionKey: exampleKey,
			transformEvent: func(ce *cloudevents.Event) {
				correlationId := ce.Extensions()["correlationid"]
				parts := strings.Split(correlationId.(string), ":")
				ce.SetExtension("replyid", fmt.Sprintf("otherid:%s", parts[1]))
			},
			expectValid: false,
		},
		"different correlationid and replyid attribute names": {
			encryptionKey:     exampleKey,
			decryptionKey:     exampleKey,
			replyIdName:       "reply",
			correlationIdName: "correlate",
			transformEvent: func(ce *cloudevents.Event) {
				ce.SetExtension("reply", ce.Extensions()["correlate"])
			},
			expectValid: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.replyIdName == "" {
				tc.replyIdName = "replyid"
			}

			if tc.correlationIdName == "" {
				tc.correlationIdName = "correlationid"
			}

			ce := cloudevents.NewEvent()
			ce.SetID("exampleid")

			err := SetCorrelationId(&ce, tc.correlationIdName, tc.encryptionKey)
			if tc.expectEncryptionError {
				assert.Error(t, err, "setting correlationid should fail")
				return
			}

			assert.NoError(t, err, "setting correlationid should not fail")

			tc.transformEvent(&ce)

			valid, err := VerifyReplyId(ce.Extensions()[tc.replyIdName].(string), tc.decryptionKey)
			if tc.expectDecryptionError {
				assert.Error(t, err, "verifying replyid should produce an error")
				return
			}

			assert.NoError(t, err, "verifying replyid should not produce an error")
			assert.Equal(t, tc.expectValid, valid)
		})

	}
	t.Parallel()

}
