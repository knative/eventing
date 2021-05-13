// +build hackhackhack

/*
Copyright 2021 The Knative Authors

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

package injection

import (
	"io/ioutil"
	"log"
	"os"
)

/*
HACK HACK HACK

To work around the fact that go does not have a way to confirm files and tests
can be compiled correctly, we have a CI build step that looks for all tags, and
then attempts to run the tests with a filter that excludes all tests like this:

$ go test -vet=off -tags "${tags}" --count=1 -run=^$ ./...

This has a side-effect of also running TestMain and init functions as `go run`
is executing the code and `-run=^$` skips all tests.

The new reconciler-test e2e and conformance tests use Knative dependency
injection to pass kubeclients around, and that client is created in the
TestMain function, which then fails to run and fails the test because we have
no valid kubeconfig in the go-build actions.

To work around this, we are going to make a build tag file that will never be
used except in the case of running go test with all tags set, as we do in
go-build actions. If this file is included in the build, it will make a temp
file, and set this valid but fake kubeconfig file to the env var `KUBECONFIG`,
allowing the TestMain and init functions are allowed to run without panics.

---

To generate a new `cluster.certificate-authority-data`:

$ openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 3650

Fill in the subject details...

Then convert the cert.pem file into base64:

$ cat cert.pem | base64 -w 0

Inspecting the cert.pem file should result in:

$cat cert.pem  | openssl x509 -in - -text -noout
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            58:e1:4b:a3:f9:be:d0:88:9c:4b:5c:b6:b1:89:d1:e0:3f:ef:73:fd
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: C = US, ST = WA, L = Seattle, O = Knative, CN = knative.dev
        Validity
            Not Before: May 13 17:44:50 2021 GMT
            Not After : May 11 17:44:50 2031 GMT
        Subject: C = US, ST = WA, L = Seattle, O = Knative, CN = knative.dev
...
*/

const config = `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURpVENDQW5HZ0F3SUJBZ0lVV09GTG8vbSswSWljUzF5MnNZblI0RC92Yy8wd0RRWUpLb1pJaHZjTkFRRUwKQlFBd1ZERUxNQWtHQTFVRUJoTUNWVk14Q3pBSkJnTlZCQWdNQWxkQk1SQXdEZ1lEVlFRSERBZFRaV0YwZEd4bApNUkF3RGdZRFZRUUtEQWRMYm1GMGFYWmxNUlF3RWdZRFZRUUREQXRyYm1GMGFYWmxMbVJsZGpBZUZ3MHlNVEExCk1UTXhOelEwTlRCYUZ3MHpNVEExTVRFeE56UTBOVEJhTUZReEN6QUpCZ05WQkFZVEFsVlRNUXN3Q1FZRFZRUUkKREFKWFFURVFNQTRHQTFVRUJ3d0hVMlZoZEhSc1pURVFNQTRHQTFVRUNnd0hTMjVoZEdsMlpURVVNQklHQTFVRQpBd3dMYTI1aGRHbDJaUzVrWlhZd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUURUClgwR1FNOTA5Q0ZkV1NxNmMveUc2QkZIc1FrWW1ab3RFMFlGRHNMUEVXU3ByemRWVzduQmYydWJyVUdJcnl5OU0KWk9pSklET0RlRUZXZkNMbm5rT1hTUEVrWjVyMjZ5dlkrS0hCVjNRRmJ1VThDR21XcVAycm5CTVd1bU1MN0JiYQpvOG9LTE9VSk9VdDZzK2huUGN1UkhybVpWZE9KanBIL1FSaFpoMVh6TUVJYW1aam5hUm9KeFAxaVVLN0pybm9ZCjE2bkhIWERjVkI3SitqQ1FzWk42WXFtTU11VVM5WGJvV3R1YkprK0tTQUM3SnFtT1BxekQxaFg0T0gwU2xDc3YKQTNtSDBzcGRLREordjRDWU1HSkM0eGFPanFUbkVpeGZuSHpsVjhCRFNRZnkreDN4WHRtTitKbGVURjc4WXpoYgp6QlF2QWpPUzZrODQyWmxheTU2bEFnTUJBQUdqVXpCUk1CMEdBMVVkRGdRV0JCVGVZalRFK0tndERMYjA2cTdvCk95dVRsNWFQWHpBZkJnTlZIU01FR0RBV2dCVGVZalRFK0tndERMYjA2cTdvT3l1VGw1YVBYekFQQmdOVkhSTUIKQWY4RUJUQURBUUgvTUEwR0NTcUdTSWIzRFFFQkN3VUFBNElCQVFBdVZzTldIakJrSmdtT3ZPSUxFS202VmRpSwpzcEpwS0V6a1hQcVR6UHpsR3RDUWxha2tBdmd6RGs4WTVSYk9BWFRHMDJiVW5ZWlg5WnBRZFh4UHZzVGtNNlhWCnMxK1N3MWlQbDNJVXEvSTBGbEVsMmhpQjArM1o5S0RvOE9XWk90VmlsbWN6bGZOMmZqamQ4R3U0NE1WOFkvRDAKWC9oVE9vSWcyYnJrWTd4NDBCbVpwVG90S3lLWHdiemFtWGtWd1p3ZXlKSWhhbk00N1Vhd0xqVFc0L3VOUVZHVwpQcVJEcWJQeFVtMkRBdDJyMm4vams2em1TVzBVNzUvM3kwenhKMG1OcWV5Q1RFcGszSHl1U0xlVUtMZ1p2MmlqClJtcmp0NmtBbGVabCsyWUVSMzg1M1U4djNuQXhFR0tiMmhjLzdEZ3NQRUVHS3MzRENIUU9TTnNiTGpheQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    server: https://127.0.0.1:6443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
current-context: default
kind: Config
preferences: {}
users:
- name: default
  user:
	password: fakefakefake
	username: admin
`

func init() {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "fake-kubeconfig-")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}

	log.Println("Created fake kubeconfig file: " + tmpFile.Name())

	if _, err = tmpFile.Write([]byte(config)); err != nil {
		log.Fatal("Failed to write to temporary file", err)
	}

	if err := tmpFile.Close(); err != nil {
		log.Fatal(err)
	}

	os.Setenv("KUBECONFIG", tmpFile.Name())
}
