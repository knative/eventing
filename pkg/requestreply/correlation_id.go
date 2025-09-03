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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

/*
 The RequestReply resource uses a correlation id set as a cloudevents extension to track which cloudevent is a reply to the initial request

 This works through setting an extension attribute (by default correlationid) to contain the correlationid, and then when the final service
 processing the request determines that their event should be treated as the "reply", the correlationid attribute is copied into a reply
 cloudevent extension attribute (by default replyid).

 The format of the correlationid/replyid attribute is: <original event id>:<base64 encoding of AES encrypted original event id>:<idx>

 The AES encryption of the original id is done to ensure that the correlation id was created by the RequestReply resource, rather than a
 third party. The idx is used for routing to ensure reply events do not overwhelm pods
*/

// VerifyReplyId takes the reply id from a cloudevent and checks that it is valid for this RequestReply resource, by checking that the decrypted id matches the unencrypted id
func VerifyReplyId(replyId string, aesKey []byte) (bool, error) {
	parts := strings.Split(replyId, ":")
	if len(parts) != 3 {
		return false, fmt.Errorf("expected three parts in the replyid attribute, had %d", len(parts))
	}

	originalId := parts[0]
	encryptedId := parts[1]

	encryptedBytes, err := base64.URLEncoding.DecodeString(encryptedId)
	if err != nil {
		return false, fmt.Errorf("failed to decode base64 data in replyid: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return false, fmt.Errorf("failed to create aes cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return false, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(encryptedBytes) < gcm.NonceSize() {
		return false, fmt.Errorf("the encrypted data is too short to be valid")
	}

	nonce, cipherText := encryptedBytes[:gcm.NonceSize()], encryptedBytes[gcm.NonceSize():]
	decryptedId, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return false, fmt.Errorf("failed to decrypt the data: %w", err)
	}

	return string(decryptedId) == originalId, nil
}

// SetCorrelationId sets the correlationid for a cloudevent by encrypting the id and setting the correlationid attribute with the original and encrypted ids
func SetCorrelationId(ce *cloudevents.Event, correlationIdName string, aesKey []byte, idx int) error {
	id := ce.ID()

	idBytes := []byte(id)

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return fmt.Errorf("failed to create aes cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to read random data for gcm nonce: %w", err)
	}

	encryptedIdBytes := gcm.Seal(nonce, nonce, idBytes, nil)

	encryptedId := base64.URLEncoding.EncodeToString(encryptedIdBytes)

	ce.SetExtension(correlationIdName, fmt.Sprintf("%s:%s:%d", id, encryptedId, idx))

	return nil
}
