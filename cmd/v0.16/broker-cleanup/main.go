/*
Copyright 2020 The Knative Authors
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
	"math"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection/clients/dynamicclient"
)

type envConfig struct {
	SystemNamespace        string `envconfig:"SYSTEM_NAMESPACE" default:"knative-eventing"`
	ReplacementBrokerClass string `envconfig:"REPLACEMENT_BROKER_CLASS" default:"MTChannelBroker"`
	DryRun                 bool   `envconfig:"DRY_RUN" default:"false"`
}

func main() {
	ctx := injectionEnabled()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Printf("[ERROR] Failed to process env var: %s", err)
		os.Exit(1)
	}

	k8s := kubeclient.Get(ctx)
	client := eventingclient.Get(ctx)

	nss, err := k8s.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	var cleanups []corev1.ObjectReference
	relabel := make([]v1beta1.Broker, 0)

	for i := 0; i < 16; i++ {
		// Reset.
		cleanups = make([]corev1.ObjectReference, 0)

		interrupted := false

		for _, name := range []string{"broker-controller", "broker-filter", "broker-ingress"} {

			if d, err := k8s.AppsV1().Deployments(env.SystemNamespace).Get(name, metav1.GetOptions{}); err != nil {
				fmt.Printf("# [error] %s\n", err)
			} else if isScaledZero(d) {
				d.Kind = "Deployment"
				d.APIVersion = "apps/v1"

				fmt.Printf("---\n\n%s\n\n", toYaml(d))

				cleanups = append(cleanups, corev1.ObjectReference{
					Kind:       d.Kind,
					Namespace:  d.Namespace,
					Name:       d.Name,
					APIVersion: d.APIVersion,
				})
			} else {
				fmt.Printf("#  Waiting until system namespace broker components are scaled to zero. [try %d/16]\n", i+1)
				interrupted = true
				break
			}
		}

		if interrupted {
			if i == 15 {
				panic("control plane is not in the expected state, is the mtbroker installed?")
			}
			time.Sleep(time.Duration(math.Exp2(float64(i))) * time.Second)
		} else {
			break
		}
	}

	for _, ns := range nss.Items {
		brokers, err := client.EventingV1beta1().Brokers(ns.Name).List(metav1.ListOptions{})
		if err != nil {
			fmt.Printf("# [error] failed to list brokers in namespace %q, %s\n", ns.Name, err)
		}

		foundBrokerForCleaning := false
		for _, broker := range brokers.Items {
			clean := false

			if broker.Annotations["eventing.knative.dev/broker.class"] == "ChannelBasedBroker" {
				fmt.Printf("# Found ChannelBasedBroker %s/%s, need to clean.\n", broker.Namespace, broker.Name)
				clean = true
				foundBrokerForCleaning = true
				relabel = append(relabel, broker)
			}

			if !clean {
				continue
			}

			// Now look for deployments.

			ingressName := fmt.Sprintf("%s-broker-ingress", broker.Name)
			filterName := fmt.Sprintf("%s-broker-filter", broker.Name)

			if ingress, err := k8s.AppsV1().Deployments(ns.Name).Get(ingressName, metav1.GetOptions{}); err != nil {
				fmt.Printf("# [error] %s\n", err)
			} else if metav1.IsControlledBy(ingress, &broker) {
				ingress.Kind = "Deployment"
				ingress.APIVersion = "apps/v1"

				fmt.Printf("---\n\n%s\n\n", toYaml(ingress))

				cleanups = append(cleanups, corev1.ObjectReference{
					Kind:       ingress.Kind,
					Namespace:  ingress.Namespace,
					Name:       ingress.Name,
					APIVersion: ingress.APIVersion,
				})
			} else {
				fmt.Printf("#  Found Ingress Deployment %s/%s, but not owned?\n", ingress.Namespace, ingress.Name)
			}

			if filter, err := k8s.AppsV1().Deployments(ns.Name).Get(filterName, metav1.GetOptions{}); err != nil {
				fmt.Printf("# [error] %s\n", err)
			} else if metav1.IsControlledBy(filter, &broker) {
				filter.Kind = "Deployment"
				filter.APIVersion = "apps/v1"

				fmt.Printf("---\n\n%s\n\n", toYaml(filter))

				cleanups = append(cleanups, corev1.ObjectReference{
					Kind:       filter.Kind,
					Namespace:  filter.Namespace,
					Name:       filter.Name,
					APIVersion: filter.APIVersion,
				})
			} else {
				fmt.Printf("#  Found Filter Deployment %s/%s, but not owned?\n", filter.Namespace, filter.Name)
			}
		}

		if foundBrokerForCleaning {
			// Look for Role Bindings
			if ingress, err := k8s.RbacV1().RoleBindings(ns.Name).Get("eventing-broker-ingress", metav1.GetOptions{}); err != nil {
				fmt.Printf("# [error] %s\n", err)
			} else {
				ingress.Kind = "RoleBinding"
				ingress.APIVersion = "rbac.authorization.k8s.io/v1"

				fmt.Printf("---\n\n%s\n\n", toYaml(ingress))

				cleanups = append(cleanups, corev1.ObjectReference{
					Kind:       ingress.Kind,
					Namespace:  ingress.Namespace,
					Name:       ingress.Name,
					APIVersion: ingress.APIVersion,
				})
			}

			if filter, err := k8s.RbacV1().RoleBindings(ns.Name).Get("eventing-broker-filter", metav1.GetOptions{}); err != nil {
				fmt.Printf("# [error] %s\n", err)
			} else {
				filter.Kind = "RoleBinding"
				filter.APIVersion = "rbac.authorization.k8s.io/v1"

				fmt.Printf("---\n\n%s\n\n", toYaml(filter))

				cleanups = append(cleanups, corev1.ObjectReference{
					Kind:       filter.Kind,
					Namespace:  filter.Namespace,
					Name:       filter.Name,
					APIVersion: filter.APIVersion,
				})
			}

			// Look for Service Accounts
			if ingress, err := k8s.CoreV1().ServiceAccounts(ns.Name).Get("eventing-broker-ingress", metav1.GetOptions{}); err != nil {
				fmt.Printf("# Warn: Skipping Service Account %s/%s, %s\n", ns.Name, "eventing-broker-ingress", err)
			} else {
				ingress.Kind = "ServiceAccount"
				ingress.APIVersion = "v1"

				fmt.Printf("---\n\n%s\n\n", toYaml(ingress))

				cleanups = append(cleanups, corev1.ObjectReference{
					Kind:       ingress.Kind,
					Namespace:  ingress.Namespace,
					Name:       ingress.Name,
					APIVersion: ingress.APIVersion,
				})
			}

			if filter, err := k8s.CoreV1().ServiceAccounts(ns.Name).Get("eventing-broker-filter", metav1.GetOptions{}); err != nil {
				fmt.Printf("# Warn: Skipping Service Account %s/%s, %s\n", ns.Name, "eventing-broker-filter", err)
			} else {
				filter.Kind = "ServiceAccount"
				filter.APIVersion = "v1"

				fmt.Printf("---\n\n%s\n\n", toYaml(filter))

				cleanups = append(cleanups, corev1.ObjectReference{
					Kind:       filter.Kind,
					Namespace:  filter.Namespace,
					Name:       filter.Name,
					APIVersion: filter.APIVersion,
				})
			}
		}
	}

	if !env.DryRun {
		for _, b := range relabel {
			b.Annotations["eventing.knative.dev/broker.class"] = env.ReplacementBrokerClass
			if _, err := client.EventingV1beta1().Brokers(b.Namespace).Update(&b); err != nil {
				fmt.Printf("# [error] failed to update broker class for %s/%s: %s\n", b.Namespace, b.Name, err)
			}
		}

		dynamic := dynamicclient.Get(ctx)
		for _, ref := range cleanups {
			fmt.Printf("# will delete %v\n", ref)
			gvr, _ := meta.UnsafeGuessKindToResource(ref.GroupVersionKind())
			if err := dynamic.Resource(gvr).Namespace(ref.Namespace).Delete(ref.Name, nil); err != nil {
				fmt.Printf("# [error] failed to delete %s %s\n", ref.String(), err)
			}
		}

	}
	fmt.Printf("# Done, cleaned %d resources.\n", len(cleanups))
}

func isScaledZero(deploy *appsv1.Deployment) bool {
	return deploy.Status.Replicas == 0 && deploy.Status.ReadyReplicas == 0
}

func toYaml(yam interface{}) string {
	b, err := yaml.Marshal(yam)
	if err != nil {
		return fmt.Sprintf("# Warn: Failed to convert resource into yaml, %s\n", err)
	}
	return string(b)
}
