/*
Copyright 2022 The Knative Authors

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

package subscriptionsapi

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/rc4"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path"
	"regexp"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"knative.dev/eventing/pkg/eventfilter"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	cesql "github.com/cloudevents/sdk-go/sql/v2"
	cefn "github.com/cloudevents/sdk-go/sql/v2/function"
	ceruntime "github.com/cloudevents/sdk-go/sql/v2/runtime"
)

// Add a user defined function to validate correlation id, then return a cesql_filter
func NewCESQLCorrelationIdFilter(expr string) (eventfilter.Filter, error) {

	var correlationIdFilterFunction = cefn.NewFunction(
		"KN_VERIFY_CORRELATIONID",
		[]cesql.Type{cesql.StringType},
		cesql.TypePtr(cesql.StringType),
		cesql.BooleanType,
		func(event cloudevents.Event, i []interface{}) (interface{}, error) {
			correlationId := i[0].(string)

			match, _ := regexp.MatchString(".*:.*", correlationId)
			if !match {
				return false, errors.New("correlationId Format: <original id>:<base64/hex encoded encrypted id>")
			}

			slice := strings.Split(correlationId, ":")
			originalId := slice[0]
			encryptedId := slice[1]

			encryptedIdBytes, err := decodeBase64OrHex(encryptedId)
			if err != nil {
				return false, err
			}

			// Create a set of secret names to try looking for in k8s
			secretNamesToTry := make(map[string]bool)

			// Iterate through secret names in argument list and add them to the set
			for num, secret := range i {
				if num != 0 {
					secretNamesToTry[secret.(string)] = true
				}
			}

			secrets, err := getSecretsFromK8s()
			if err != nil {
				return false, err
			}

			/*
			 * Go through each retrived secret
			 * Check if the secret has data for a key and algorithm
			 * Check if the secret name matches with one of the arguments
			 * Decrypt encryptedId using the current secret's key
			 * Check if the encrypted value matches with originalId
			 */
			for _, secret := range secrets {
				key, keyFieldExists := secret.Data["key"]
				algorithm, algorithmFieldExists := secret.Data["algorithm"]

				if keyFieldExists && algorithmFieldExists && secretNamesToTry[secret.Name] {
					var decryptionFunc func(originalId string, encryptedIdBytes []byte, key []byte) (bool, error)
					switch strings.ToUpper(string(algorithm)) {
					case "AES", "AES-ECB":
						decryptionFunc = compareWithAES
					case "DES":
						decryptionFunc = compareWithDES
					case "3DES", "TRIPLEDES":
						decryptionFunc = compareWithTripleDES
					case "RC4":
						decryptionFunc = compareWithRC4
					default:
						return false, errors.New("cipher algorithm not supported")
					}

					res, err := decryptionFunc(originalId, encryptedIdBytes, key)
					if err != nil {
						return false, err
					}
					if res {
						return true, nil
					}
				}
			}

			return false, nil
		},
	)

	ceruntime.AddFunction(correlationIdFilterFunction)

	return NewCESQLFilter(expr)
}

func getSecretsFromK8s() ([]v1.Secret, error) {
	// Assuming .kube/config is in the home directory
	basePath, _ := os.UserHomeDir()
	defaultKubeConfigPath := path.Join(basePath, ".kube", "config")

	// Set up k8s client to get secrets
	config, err := clientcmd.BuildConfigFromFlags("", defaultKubeConfigPath)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	secrets, err := clientset.CoreV1().Secrets("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return secrets.Items, nil
}

func decodeBase64OrHex(encryptedId string) ([]byte, error) {
	hexRegex := regexp.MustCompile("^([0-9A-Fa-f]+)$")
	base64Regex := regexp.MustCompile(`^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{4})$`)

	encryptedIdBytes := []byte(encryptedId)

	if hexRegex.Match(encryptedIdBytes) {
		return hex.DecodeString(encryptedId)
	} else if base64Regex.Match(encryptedIdBytes) {
		return base64.StdEncoding.DecodeString(encryptedId)
	} else {
		return nil, errors.New("encryptedId must either be Base64 or Hex encoded")
	}
}

func compareWithAES(originalId string, encryptedIdBytes []byte, key []byte) (bool, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return false, err
	}
	plainText := getPlaintextFromBlock(block, encryptedIdBytes)

	return plainText == originalId, nil
}

func compareWithDES(originalId string, encryptedIdBytes []byte, key []byte) (bool, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return false, nil
	}
	plainText := getPlaintextFromBlock(block, encryptedIdBytes)

	return plainText == originalId, nil
}

func compareWithTripleDES(originalId string, encryptedIdBytes []byte, key []byte) (bool, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return false, nil
	}
	plainText := getPlaintextFromBlock(block, encryptedIdBytes)

	return plainText == originalId, nil
}

func compareWithRC4(originalId string, encryptedIdBytes []byte, key []byte) (bool, error) {
	cipher, err := rc4.NewCipher(key)
	if err != nil {
		return false, nil
	}
	out := make([]byte, len(encryptedIdBytes))
	cipher.XORKeyStream(out, encryptedIdBytes)
	plainText := string(out)

	return plainText == originalId, nil
}

func getPlaintextFromBlock(block cipher.Block, encryptedIdBytes []byte) string {
	plainText := make([]byte, len(encryptedIdBytes))
	for i, j := 0, block.BlockSize(); i < len(encryptedIdBytes); i, j = i+block.BlockSize(), j+block.BlockSize() {
		block.Decrypt(plainText[i:j], encryptedIdBytes[i:j])
	}
	trim := 0
	if len(plainText) > 0 {
		trim = len(plainText) - int(plainText[len(plainText)-1])
	}

	if trim >= 0 {
		trimmedPlaintext := string(plainText[:trim])

		return trimmedPlaintext
	}

	return ""
}
