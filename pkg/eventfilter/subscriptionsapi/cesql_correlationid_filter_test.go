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
	"errors"
	"os"
	"path"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/eventing/pkg/eventfilter"
)

// Info to create k8s secret object during testing
type Secret struct {
	Name      string
	Key       string
	Algorithm string
}

var secrets = [5]Secret{
	{
		Name:      "secret1",
		Key:       "aesEncryptionKey",
		Algorithm: "AES",
	},
	{
		Name:      "secret2",
		Key:       "abcabcdefdefmnop",
		Algorithm: "AES-ECB",
	},
	{
		Name:      "secret3",
		Key:       "desEncKe",
		Algorithm: "DES",
	},
	{
		Name:      "secret4",
		Key:       "tripleDesKeyForEncrypter",
		Algorithm: "3DES",
	},
	{
		Name:      "secret5",
		Key:       "rc4EncKey",
		Algorithm: "RC4",
	},
}

func TestCESQLCorrelationIdFilter(t *testing.T) {
	tests := map[string]struct {
		expression string
		event      *cloudevents.Event
		want       eventfilter.FilterResult
	}{
		"1. CorrelationId encoded in hex matches with 'secret1'": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:2826A47C1C3325A5899235911B6F546F', 'secret1')",
			want:       eventfilter.PassFilter,
		},
		"2. CorrelationId encoded in base64 matches with 'secret1'": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:KCakfBwzJaWJkjWRG29Ubw==', 'secret1')",
			want:       eventfilter.PassFilter,
		},
		"3. CorrelationId encoded in hex matches with 'secret2": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:dcd075b3679cf81325a09e0786b87f87', 'secret2')",
			want:       eventfilter.PassFilter,
		},
		"4. CorrelationId encoded in base64 matches with 'secret2'": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:3NB1s2ec+BMloJ4Hhrh/hw==', 'secret2')",
			want:       eventfilter.PassFilter,
		},
		"5. CorrelationId encoded in base64 matches with 'secret3'": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:qpCNy1Dy5aXWnXzgiAfg6w==', 'secret3')",
			want:       eventfilter.PassFilter,
		},
		"6. CorrelationId encoded in hex matches with 'secret3'": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:aa908dcb50f2e5a5d69d7ce08807e0eb', 'secret3')",
			want:       eventfilter.PassFilter,
		},
		"7. CorrelationId encoded in base64 matches with 'secret4'": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:2jA1dTIjigEqR5eOruRjxA==', 'secret4')",
			want:       eventfilter.PassFilter,
		},
		"8. CorrelationId encoded in hex matches with 'secret4": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:da30357532238a012a47978eaee463c4', 'secret4')",
			want:       eventfilter.PassFilter,
		},
		"9. CorrelationId encoded in base64 matches with 'secret5'": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:Jl9EcLh2qv8t', 'secret5')",
			want:       eventfilter.PassFilter,
		},
		"10. CorrelationId encoded in hex matches with 'secret5": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:265f4470b876aaff2d', 'secret5')",
			want:       eventfilter.PassFilter,
		},
		"11. CorrelationId encoded in hex fails to match for 'secret2' and 'secret3'": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:2826A47C1C3325A5899235911B6F546F', 'secret2', 'secret3')",
			want:       eventfilter.FailFilter,
		},
		"12. CorrelationId encoded in base64 fails to match for 'secret2' and 'secret3'": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:KCakfBwzJaWJkjWRG29Ubw==', 'secret2', 'secret3')",
			want:       eventfilter.FailFilter,
		},
		"13. CorrelationId encoded in base64 fails to match for 'secret6'": {
			expression: "KN_VERIFY_CORRELATIONID('randomId1:KCakfBwzJaWJkjWRG29Ubw==', 'secret6')",
			want:       eventfilter.FailFilter,
		},
		"14. CorrelationId encoded in hex matches with 'secret2'": {
			expression: "KN_VERIFY_CORRELATIONID('97cdf5c1-7826-4e4b-9406-c1d9f18b7740:349c52325d623791549f83a3cffb328bd68a3e2f641227b804c2570c39a6b4d3c7bc25b1e39b5c5539eedfb21c6b0084', 'secret2')",
			want:       eventfilter.PassFilter,
		},
		"15. CorrelationId encoded in base64 matches with 'secret2'": {
			expression: "KN_VERIFY_CORRELATIONID('97cdf5c1-7826-4e4b-9406-c1d9f18b7740:NJxSMl1iN5FUn4Ojz/syi9aKPi9kEie4BMJXDDmmtNPHvCWx45tcVTnu37IcawCE', 'secret2')",
			want:       eventfilter.PassFilter,
		},
		"16. CorrelationId encoded in base64 matches with 'secret1'": {
			expression: "KN_VERIFY_CORRELATIONID('b2e5c373-4454-46d7-805d-e2038263ae3e:/deOV4nw7c8xJm7KjDVDNtavo4XtiHe1acaLbooTJ13AXgORw/PWsDdhHb3qkXBI', 'secret1')",
			want:       eventfilter.PassFilter,
		},
		"17. CorrelationId encoded in hex matches with 'secret4'": {
			expression: "KN_VERIFY_CORRELATIONID('c59d9e75-12dd-4ff6-b721-2c5418c92c94:e3014db9448418d55189bfc6ad4533071c2a1cc2eb3698dc1c061e8b06c240f64aca74132532d4bc', 'secret4')",
			want:       eventfilter.PassFilter,
		},
	}

	deleteK8TestSecrets()

	err := createK8TestSecrets()
	if err != nil {
		t.Fatalf("Error creating k8s secrets for testing. %v", err)
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := tt.event
			if e == nil {
				e = makeEvent()
			}
			f, err := NewCESQLCorrelationIdFilter(tt.expression)
			if err != nil {
				t.Fatalf("Error instanciating CESQL CorrelationId filter. %v", err)
			}
			if got := f.Filter(context.TODO(), *e); got != tt.want {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}

	err = deleteK8TestSecrets()
	if err != nil {
		t.Fatalf("Error deleting k8s secrets for testing. %v", err)
	}
}
func initClient() (kubernetes.Clientset, error) {
	clientset := &kubernetes.Clientset{}

	// Assuming .kube/config is in the home directory
	basePath, _ := os.UserHomeDir()
	defaultKubeConfigPath := path.Join(basePath, ".kube", "config")

	// Set up k8s client to get secrets
	config, err := clientcmd.BuildConfigFromFlags("", defaultKubeConfigPath)
	if err != nil {
		return *clientset, err
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return *clientset, err
	}

	return *clientset, nil
}

func createK8TestSecrets() error {
	clientset, err := initClient()
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		_, err := clientset.CoreV1().Secrets("default").Get(context.TODO(), secret.Name, metav1.GetOptions{})
		if err == nil {
			return errors.New(secret.Name + " already exists")
		}

		data := map[string][]byte{
			"key":       []byte(secret.Key),
			"algorithm": []byte(secret.Algorithm),
		}
		objectMetadata := metav1.ObjectMeta{Name: secret.Name}
		k8secret := &v1.Secret{Data: data, ObjectMeta: objectMetadata}

		_, err = clientset.CoreV1().Secrets("default").Create(context.TODO(), k8secret, metav1.CreateOptions{})

		if err != nil {
			return err
		}
	}

	return nil
}

func deleteK8TestSecrets() error {
	clientset, err := initClient()
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		_ = clientset.CoreV1().Secrets("default").Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
	}

	return nil
}
