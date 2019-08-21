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

package logconfig

import (
	"fmt"
	"os"
)

const (
	// Named Loggers are used to override the default log level. config-logging.yaml will use the follow:
	//
	// loglevel.controller: "info"
	// loglevel.webhook: "info"

	// Controller is the name of the override key used inside of the logging config for Controller.
	Controller = "controller"

	// Webhook is the name of the override key used inside of the logging config for Webhook Controller.
	WebhookNameEnv = "WEBHOOK_NAME"
)

func WebhookName() string {
	if webhook := os.Getenv(WebhookNameEnv); webhook != "" {
		return webhook
	}

	panic(fmt.Sprintf(`The environment variable %q is not set

If this is a process running on Kubernetes, then it should be using the downward
API to initialize this variable via:

  env:
  - name: WEBHOOK_NAME
    value: webhook
`, WebhookNameEnv))
}
