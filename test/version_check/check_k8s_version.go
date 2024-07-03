/*
Copyright 2024 The Knative Authors

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

package main

import (
	"fmt"
	"log"

	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/test"
	"knative.dev/pkg/version"
)

func main() {
	config, err := test.Flags.GetRESTConfig()
	if err != nil {
		log.Fatalf("Failed to create REST config: %v\n", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v\n", err)
	}

	err = version.CheckMinimumVersion(kubeClient.Discovery())
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("Kubernetes version is compatible.")
}
