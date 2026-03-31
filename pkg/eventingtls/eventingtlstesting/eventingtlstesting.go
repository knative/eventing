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

const (
	IssuerKind = "ClusterIssuer"
	IssuerName = "knative-eventing-ca-issuer"
)

func init() {
	CA, Key, Crt = loadCerts()
}

func StartServer(ctx context.Context, t *testing.T, port int, handler http.Handler, receiverOptions ...kncloudevents.HTTPEventReceiverOption) (string, int) {
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

	receiver := kncloudevents.NewHTTPEventReceiver(port,
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

	return string(CA), receiver.GetPort()
}

func loadCerts() ([]byte, []byte, []byte) {
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
		IP.1 = 127.0.0.1
	*/
	return []byte(`
-----BEGIN CERTIFICATE-----
MIIDPzCCAiegAwIBAgIUV9yfCLRw+lLLDI+zQjV9h/M87qYwDQYJKoZIhvcNAQEL
BQAwLzELMAkGA1UEBhMCVVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290
LUNBMB4XDTI2MDMzMTE0MzE1NloXDTM2MDMyODE0MzE1NlowLzELMAkGA1UEBhMC
VVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290LUNBMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEAo2k0CNBAkhRey7BJ/t/IWiKYn3ivKtcNkcHS
F0n617PhSIg5ttjdEx4xy9XI6qz3E4IrX7qyZtK/sksbX2SQZ9ZUX1wAL1TJr2bK
nGGBcfxaM1ipsnzU6KmmtFAyvhG20ZGJj+3AJFj8ZMQCgttL6955lg/3T4YQjLOl
byKuS4YKvfbID8hWub+gRvls4ObrJTPACNbv8UnVxSsm4ZpQSrjHifPb+aAkCZ/m
dVa7tZFNZTULEa/KK1KJi96kO+b620BP33grt4u5YNNJLkJmQCwYgqlKyObx3lXy
8ciz6Dpyh9gp/GuJz//O3uOfqA1Kt6SWykOk85CpHjbYtESifwIDAQABo1MwUTAd
BgNVHQ4EFgQU/9K8+VgUmXHaSkcyOcbPyOEbLxMwHwYDVR0jBBgwFoAU/9K8+VgU
mXHaSkcyOcbPyOEbLxMwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOC
AQEAblgdceDReGRHLzJ0D96W8xDPNJK5MopErmc5tI817o5DpdrPxA9j2aARCB0+
iQ9ohmIXHVexgb1Fe6YG6bN3rMlccq+ax8/25JXFOQg2gSIjFZtb9iV0JjTu5mPz
zf+oJb005rD/LBVTA00VoHGttzTkT9tqXdGVWCfFsvUNP7ARslf1Gp8TcagNqNww
XAjIQGN2U9k/o4l+dWgWe/h03bC6/EoxXm/usv2mET1F6LmAh8+G1aPxLzYs+503
bHagSQAfDLoeGOyN7XatUW6laMRWZVDG1AgnJKXqFEM89d0bnHZcfMYcRtf5ED7t
WdET+xQq+T39uQw7WSnw2ncP2g==
-----END CERTIFICATE-----`), []byte(`
-----BEGIN PRIVATE KEY-----
MIIEuwIBADANBgkqhkiG9w0BAQEFAASCBKUwggShAgEAAoIBAQC8sUhKiN5hwYqq
8PcT3ovO+HUPEDvkHudDbEHx8stb9/qWiWpnS/1a9Mbghmb2PnYpD0UEXQ4QaVWq
PlpNPN92VHENs+8yOajQwsxqlCVey3JfRI0U806Mz7ia41IFsKfhOohC3rV3qxcu
L1hB5cLYBwvasJK5J3Pdk4jEDY6Ak9e4OszfgsGrpbENh3B2M3EqYvnXXsyy/Q5X
gNhrvqS1nMd4l59MSAU03uOsk8ev3Gw1mZKJdWNVdyJe8pQpvbH3ahTFm9vI7nAV
7XDoUJhuUbIHJYbhBLD2rjm9g3Iigjqa3hfMI4G9rkeUfaK99DlhEd2npjJrg9G+
hwGkGOM1AgMBAAECggEAHWV5H7OHAa/HTK5Rr9TB7zKh+gDLc9SkrspU077Bk8hk
T8OEwicCh4MO4LfPnplIi0kHtZBRupjOccFZDCNppOOu4TWhFDALbsqKUihWUhhb
7x+c4yCsoh9SYT787koBPYOC6vgLSWNsLxPNKicDXehrHlzX3uSYlnJ/ohuCkeEx
/IjY7c2+40QZzrmCGn4j3Iz3oEo7N+GLZvfYk2vO9jA18Xi9KEhPIKdZV7kYXvdJ
6mt9Xbf5Q05li8m+ibRnqVPnqjZEzkLOde53EyaDEOO5jKrAVQyoQJRFeapoPsv/
zPXWoxcufTRl/u4fJc2DiW1JS0Z7PBZU36aHvKW7GQKBgQDivmHq8y6OEqW29SS5
kzq6XcnA0+P8H10OOIiy46H1eMLCWgy98CCp/sYZ3kxTrg3Bkv29HKJi5UobpJ3s
YXxTf2o3vPrhH4bLRn7PWqQX1eRvLLb9j92DwbEnmTmLNtezk5L3zhtFY+aNFF0z
z8F1W1lGfmpayuFAVKWYF2FKxwKBgQDVCgYUDcKOvUgPsBjgm9trXGYrMREeDQL1
U5egDxkQeP+ge39uWPUsjPB4Y/TosoiW1Z9/8ce+g1ctr/fTRgALIZtxxDrnl7Z0
5scDzQUObQenCXZIu5cSZcS0UYYuTLG7zyNRg6DLDyOq+Mb4ImJ41BrpFkiqyClj
lGuIQp0GIwJ/braHGTGkibqRL8SDKhm5k0Y4PO85YuHtjgQMu0xqyHEup7dQgW2+
hULhd8AThMh37wzW2IAiyicrCFRYyBLRofOU6aJc3Y+HQboqMRURCpJl9+LddvHI
N41b8vneHxuoNwbXvCfnKPqemERZPfMzgxoXfVQ8mhh14/Nw59KCrwKBgQCIFF0J
ljh+gL109+EMJ8Iic+T3FeJ/NYR8PCcQIFS8Ru3SDtC5Ja2GBYjc/cxEjzXcUxwM
193k/XRERLCijYYdlhv6sYOGx0vOpFLfRKhELLTEp7CciObY45SgMarqDCdDde5z
dQbSbhs0bLB7c0s/Lwz5cjh8jlqRtw5w/UvbzQKBgF1rjeuulwrPlod0KTeQ0FHf
ysvAHg1UBCV9fayBgX7vE0FPlA/uobFEXQi0TvljdjrCbaNgysIBOivcvDSt99Dr
XLSh6kIO/8KJJuy//aXnbPYhJBVTSMYR2JzgGdvL6FCSL5E7DlFO5MRER/AoOV+S
P79jgoIQNwd52uB3mBP7
-----END PRIVATE KEY-----`), []byte(`
-----BEGIN CERTIFICATE-----
MIIDoDCCAoigAwIBAgIUMljegDic8ulg05kFdtpo4YFPTFswDQYJKoZIhvcNAQEL
BQAwLzELMAkGA1UEBhMCVVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290
LUNBMB4XDTI2MDMzMTE0MzE1NloXDTM2MDMyODE0MzE1NlowbTELMAkGA1UEBhMC
VVMxEjAQBgNVBAgMCVlvdXJTdGF0ZTERMA8GA1UEBwwIWW91ckNpdHkxHTAbBgNV
BAoMFEV4YW1wbGUtQ2VydGlmaWNhdGVzMRgwFgYDVQQDDA9sb2NhbGhvc3QubG9j
YWwwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC8sUhKiN5hwYqq8PcT
3ovO+HUPEDvkHudDbEHx8stb9/qWiWpnS/1a9Mbghmb2PnYpD0UEXQ4QaVWqPlpN
PN92VHENs+8yOajQwsxqlCVey3JfRI0U806Mz7ia41IFsKfhOohC3rV3qxcuL1hB
5cLYBwvasJK5J3Pdk4jEDY6Ak9e4OszfgsGrpbENh3B2M3EqYvnXXsyy/Q5XgNhr
vqS1nMd4l59MSAU03uOsk8ev3Gw1mZKJdWNVdyJe8pQpvbH3ahTFm9vI7nAV7XDo
UJhuUbIHJYbhBLD2rjm9g3Iigjqa3hfMI4G9rkeUfaK99DlhEd2npjJrg9G+hwGk
GOM1AgMBAAGjdjB0MB8GA1UdIwQYMBaAFP/SvPlYFJlx2kpHMjnGz8jhGy8TMAkG
A1UdEwQCMAAwCwYDVR0PBAQDAgTwMBoGA1UdEQQTMBGCCWxvY2FsaG9zdIcEfwAA
ATAdBgNVHQ4EFgQU+XEL9zay5YEXxsx3UMWycZLJaUYwDQYJKoZIhvcNAQELBQAD
ggEBAJodNtc9M20guT4f4pcCeTsrl+QtVWQ/vsozGo0UCprPZ8GKcpDpCcn5P3ec
YorT1tYU8rzWNprpUzOvzWJZB+UYk0yShWwvTiOLSZ44FXYxykMMFqtkPEunVrpm
u+rIp5zAjQP1l5oOjUBbly+bCy09aKSKTkpLeyMdS1oHzglEleFhSmygRX6rfuRu
j9hPHXZiZkGXxeZmBdAe3q1SFaI1sAQk0Kpdexb3l5YsxKjpMqTDb0/I4UxH1RJR
FpngwKBkOkoHCi5R8eKOL9VJ25MTOYW682TNrklpxDzHqj3XKmhhBavNS3PnUX2e
7xeYxFOnzNDuuuLywYTj+HsX7nA=
-----END CERTIFICATE-----`)
}
