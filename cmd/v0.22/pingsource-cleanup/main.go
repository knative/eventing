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

package main

import (
	"fmt"
	"log"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/kelseyhightower/envconfig"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	sourcesv1beta2 "knative.dev/eventing/pkg/apis/sources/v1beta2"
)

type envConfig struct {
	SystemNamespace string `envconfig:"SYSTEM_NAMESPACE" default:"knative-eventing"`
	DryRun          bool   `envconfig:"DRY_RUN" default:"false"`
}

func main() {
	ctx := injectionEnabled()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("[ERROR] Failed to process env var: %s", err)
	}

	k8s := kubeclient.Get(ctx)
	client := eventingclient.Get(ctx)

	nss, err := k8s.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("[ERROR] Failed to list namespaces: %s", err)
	}

	var cleanups []sourcesv1beta2.PingSource

	for _, ns := range nss.Items {
		fmt.Printf("# processing namespace %s\n", ns.Name)

		pingsources, err := client.SourcesV1beta2().PingSources(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("# [error] failed to list pingsources in namespace %q, %s\n", ns.Name, err)
		}

		for _, pingsource := range pingsources.Items {
			if len(pingsource.Finalizers) > 0 {
				finalizers := sets.NewString(pingsource.Finalizers...)
				if finalizers.Has("pingsources.sources.knative.dev") {
					fmt.Printf("# Found PingSource %s/%s, need to remove finalizer.\n", pingsource.Namespace, pingsource.Name)
					cleanups = append(cleanups, pingsource)
				}
			}
		}
	}

	if !env.DryRun {
		for i := range cleanups {
			ref := cleanups[i]
			fmt.Printf("# will remove finalizer for %s/%s\n", ref.Namespace, ref.Name)

			finalizers := sets.NewString(ref.Finalizers...)
			finalizers.Delete("pingsources.sources.knative.dev")
			ref.Finalizers = finalizers.List()

			if _, err := client.SourcesV1beta2().PingSources(ref.Namespace).Update(ctx, &ref, metav1.UpdateOptions{}); err != nil {
				fmt.Printf("# [error] failed to update %s/%s %s\n", ref.Namespace, ref.Name, err)
			}
		}
	}
	fmt.Printf("# Done, cleaned %d resources.\n", len(cleanups))
}
