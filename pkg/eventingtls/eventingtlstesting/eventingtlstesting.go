/*
Copyright 2023 The Knative Authors

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

package eventingtlstesting

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret/fake"

	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
)

var (
	CA  []byte
	Key []byte
	Crt []byte
)

func init() {
	CA, Key, Crt = loadCerts()
}

func StartServer(ctx context.Context, t *testing.T, port int, handler http.Handler, receiverOptions ...kncloudevents.HTTPMessageReceiverOption) string {
	secret := types.NamespacedName{
		Namespace: "knative-tests",
		Name:      "tls-secret",
	}

	_ = secretinformer.Get(ctx).Informer().GetStore().Add(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: secret.Namespace,
			Name:      secret.Name,
		},
		Data: map[string][]byte{
			"tls.key": Key,
			"tls.crt": Crt,
		},
		Type: corev1.SecretTypeTLS,
	})

	serverTLSConfig := eventingtls.NewDefaultServerConfig()
	serverTLSConfig.GetCertificate = eventingtls.GetCertificateFromSecret(ctx, secretinformer.Get(ctx), kubeclient.Get(ctx), secret)
	tlsConfig, err := eventingtls.GetTLSServerConfig(serverTLSConfig)
	assert.Nil(t, err)

	receiver := kncloudevents.NewHTTPMessageReceiver(port,
		append(receiverOptions,
			kncloudevents.WithTLSConfig(tlsConfig),
		)...,
	)

	go func() {
		err := receiver.StartListen(ctx, handler)
		if err != nil {
			panic(err)
		}
	}()

	return string(CA)
}

func loadCerts() (ca, key, crt []byte) {
	/*
		Provisioned using:
		openssl req -x509 -nodes -new -sha256 -days 1024 -newkey rsa:2048 -keyout RootCA.key -out RootCA.pem -subj "/C=US/CN=Knative-Example-Root-CA"
		openssl x509 -outform pem -in RootCA.pem -out RootCA.crt
		openssl req -new -nodes -newkey rsa:2048 -keyout localhost.key -out localhost.csr -subj "/C=US/ST=YourState/L=YourCity/O=Example-Certificates/CN=localhost.local"
		openssl x509 -req -sha256 -days 1024 -in localhost.csr -CA RootCA.pem -CAkey RootCA.key -CAcreateserial -extfile domains.ext -out localhost.crt

		Copy:
		- RootCA.crt for ca
		- localhost.key for key
		- localhost.crt for crt

		domains.ext file:
		authorityKeyIdentifier=keyid,issuer
		basicConstraints=CA:FALSE
		keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
		subjectAltName = @alt_names
		[alt_names]
		DNS.1 = localhost
	*/
	return []byte(`
-----BEGIN CERTIFICATE-----
MIIDPzCCAiegAwIBAgIUYuysnNGPwBjbiDRc+/9s9Jl3N8YwDQYJKoZIhvcNAQEL
BQAwLzELMAkGA1UEBhMCVVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290
LUNBMB4XDTIzMDQwNTEzMTQxMloXDTI2MDEyMzEzMTQxMlowLzELMAkGA1UEBhMC
VVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290LUNBMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEAyEwyvWKc/SJzblAc/pNIE7UJHIpbEUDtwOom
YvytwcMhI73zlSVhAcOagwnn3AvBg3McGPyLGghr9EuXBE1Vx584Pw1cmKOwbyiC
SQtaRwbztzM555T4Rtrk4tdKm+WHD/HiYAB/s+OnPJ6F6yBedT6nW08HlTP5lJX1
U21+OAiOSU4zx+YYlkRbHq8aYggB1YM+hdRSStl9Mc/nw6TWlVsd2LjppXgoxSKl
YTB4ZwnaKmrIRa9hFf1DVY/nTlmUP2iGr9131CLs3/5QyoFRWI6ayfnRSkmVwKLS
8AW/b4jh+qJVIaeLCw5QF4RuqsE5VaUj6wlEqWM4eI+5Uaj+5QIDAQABo1MwUTAd
BgNVHQ4EFgQUklwJ+26zi+P3w3TNBBq62yMS7zYwHwYDVR0jBBgwFoAUklwJ+26z
i+P3w3TNBBq62yMS7zYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOC
AQEAWwlatEXUTiB4O3M/fLSZ4JlAA1bq2U+dafiUiq5Ym0F1/UGu7YD74LGm4n03
X9QU4jVwAkxL8pFV68NEBFJXOwFRyVQ1THAfhzij5teMAd4aqaffEPF0YfE8+rdg
MSQx9n/OOeeyqWlaAqI3D9SEoSFPk5Xbfdzu6zGggizJwIYus77LOYxS7hvGxCci
dTnEHvGoP14/13F/2vZLSaH9qrAv3cTenVYRN1QSSVI0V2XAhz+HAOjO2muaaYEG
2eKiYvHvG0p5aCRIZYi4z3q6QAr9z+nyRyO1Tw/CnbCOeULQoOZWLy8xE9zBOE1t
JQArXobwA4IZrx13xxsMafyt0A==
-----END CERTIFICATE-----
`), []byte(`
-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQC5teo+En6U5nhq
n7ScuanqswUmPlgs9j/8l21Rhb4T+ezlYKGQGhbJyFFMuiCE1Rjn8bpCwi7Nnv12
Y2nzFhEv2Jx0yL3Tqx0Q593myqKDq7326EtbO7wmDT0XD03twH5i9XZ0L0ihPWn1
mjUyWxhnHhoFpXrsnQECJorZY6aTrFbGVYelIaj5AriwiqyL0fET8pueI2GwLjgW
HFSHX8XsGAlcLUhkQG0Z+VO9usy4M1Wpt+cL6cnTiQ+sRmZ6uvaj8fKOT1Slk/oU
eAi4WqFkChGzGzLik0QrhKGTdw3uUvI1F2sdQj0GYzXaWqRz+tP9qnXdzk1GrszK
KSlmWBTLAgMBAAECggEAM5nqhmjZJ0KKvwW1R78HCaHIkoHMOmIKEYN56qcA08gk
HPAmtEWrg1HX1Tv6gS49B2XRXW9bVeMRhm3FKLg++k5z2rdUl5X6M5JZxCEV2wRD
enG9TpJgiyouiVPFUYSlGZYe3dWtlq/b21SH54AMXcqtbFg4ubo+Z3ySJCleRbWp
iUHoTXB9oy31HMca8LaGkcsk4JlSGpThK2mF6zGI5Lz1sjYfBwrBinHImrMuH821
1JbXLjoAAoHcM/DykQe2vXe0gKJKzbScC3KAI5TimvCuwGdtfAsPWkrQoUQB8Q2N
M7DTAHqbbWGdxVWzntVb5ilFDRg3n3sVAp2AxALvcQKBgQDvlvVEyQqNDfJ8ONDS
zBoK0RN3xu8+gNYguiXMy2oJWAGmomdNLn4UWqrIRmQhBFLob+dms0V/zKn6f1y9
DZhAt86yPdi0/xXSWEw3UeAcVQcJbbUFc9GjFXjWB1nMyZgk5laBy8+ht5Vly/hC
q/oTLLM7yXVlyBZRru7VU5GDBQKBgQDGbjlarvr6PUgSQS1UJGkny815NKjsVMgG
tdA3iT7dwINxcTUbqwkp6qTKjXgeH2VsJzhWqagcE9W88uumpKIqZGKnd/vgEKss
UTVpVxzVTivcl0y70lZKC/pgycO+ML4WiI+ASPZG9W9wFNvGdWlHnPZIMFVZ/SNB
smcSAa9hjwKBgByR7Md6DccKPbswbz5j1ksp6V9kGo1igaY/bFiCfS+GDhRX02ex
vpkgwrLFKhWB1X0gMwDdKdF2j2Juo5lrsJcvE/fPRjM3I9wEaXpDSi02unMWYPq4
d+wxmEo1cDDqbTkhOnmZ2zWWlbsg2obgyR5WOz1K5bPwazDsYlCP+Y8dAoGAE2rD
yADpZEVM4SRpmBs8Av3pbFvfz8h4DlgKOPUAJtjow9gNF1kEO4rPd1aik2gFF7E6
zRgq8BxsxOGMd7ESgU1zbenKxuE6rsp/jIBOvPy6RAq2IobxlKtZY9E6i0jfwPq5
+BarqsPnlLMl0mS42Z4dZ3D7WSPxKEOZ3GQ30jcCgYBT2wsPDdGrRsBVw3C6MAhu
D8cdmsXujQXkDr51Yop7BSnFPz+Q7zWOlzwqcXEw0h/9TwWa0C5LAuKdYUkQAa26
0dsxUFD/WmdxtLqPbIwq4YkUgmYSTK0LgzDtnhYU3RwK7HZ+TMUCOxe5CrjOzlvp
uEg9BsZ6abqL1PQfsez+fw==
-----END PRIVATE KEY-----
`), []byte(`
-----BEGIN CERTIFICATE-----
MIIDmjCCAoKgAwIBAgIUYzA4bTMXevuk3pl2Mn8hpCYL2C0wDQYJKoZIhvcNAQEL
BQAwLzELMAkGA1UEBhMCVVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290
LUNBMB4XDTIzMDQwNTEzMTUyNFoXDTI2MDEyMzEzMTUyNFowbTELMAkGA1UEBhMC
VVMxEjAQBgNVBAgMCVlvdXJTdGF0ZTERMA8GA1UEBwwIWW91ckNpdHkxHTAbBgNV
BAoMFEV4YW1wbGUtQ2VydGlmaWNhdGVzMRgwFgYDVQQDDA9sb2NhbGhvc3QubG9j
YWwwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC5teo+En6U5nhqn7Sc
uanqswUmPlgs9j/8l21Rhb4T+ezlYKGQGhbJyFFMuiCE1Rjn8bpCwi7Nnv12Y2nz
FhEv2Jx0yL3Tqx0Q593myqKDq7326EtbO7wmDT0XD03twH5i9XZ0L0ihPWn1mjUy
WxhnHhoFpXrsnQECJorZY6aTrFbGVYelIaj5AriwiqyL0fET8pueI2GwLjgWHFSH
X8XsGAlcLUhkQG0Z+VO9usy4M1Wpt+cL6cnTiQ+sRmZ6uvaj8fKOT1Slk/oUeAi4
WqFkChGzGzLik0QrhKGTdw3uUvI1F2sdQj0GYzXaWqRz+tP9qnXdzk1GrszKKSlm
WBTLAgMBAAGjcDBuMB8GA1UdIwQYMBaAFJJcCftus4vj98N0zQQautsjEu82MAkG
A1UdEwQCMAAwCwYDVR0PBAQDAgTwMBQGA1UdEQQNMAuCCWxvY2FsaG9zdDAdBgNV
HQ4EFgQUnu/3vqA3VEzm128x/hLyZzR9JlgwDQYJKoZIhvcNAQELBQADggEBAFc+
1cKt/CNjHXUsirgEhry2Mm96R6Yxuq//mP2+SEjdab+FaXPZkjHx118u3PPX5uTh
gTT7rMfka6J5xzzQNqJbRMgNpdEFH1bbc11aYuhi0khOAe0cpQDtktyuDJQMMv3/
3wu6rLr6fmENo0gdcyUY9EiYrglWGtdXhlo4ySRY8UZkUScG2upvyOhHTxVCAjhP
efbMkNjmDuZOMK+wqanqr5YV6zMPzkQK7DspfRgasMAQmugQu7r2MZpXg8Ilhro1
s/wImGnMVk5RzpBVrq2VB9SkX/ThTVYEC/Sd9BQM364MCR+TA1l8/ptaLFLuwyw8
O2dgzikq8iSy1BlRsVw=
-----END CERTIFICATE-----
`)
}
