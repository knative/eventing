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

package jobsink

import (
	"context"
	"fmt"
	"sort"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/labels"
	batchlisters "k8s.io/client-go/listers/batch/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/pkg/reconciler"

	"knative.dev/eventing/pkg/apis/feature"
	sinksc "knative.dev/eventing/pkg/apis/sinks"
	sinks "knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/eventing/pkg/eventingtls"
)

const (
	maxFailedJobs = 10
)

type Reconciler struct {
	jobLister       batchlisters.JobLister
	secretLister    corev1listers.SecretLister
	systemNamespace string
}

func (r *Reconciler) ReconcileKind(ctx context.Context, js *sinks.JobSink) reconciler.Event {
	if err := r.propagateJobsStatus(js); err != nil {
		return fmt.Errorf("failed to reconcile spec.job: %w", err)
	}

	if err := r.reconcileAddress(ctx, js); err != nil {
		return fmt.Errorf("failed to reconcile address: %w", err)
	}

	return nil
}

func (r *Reconciler) propagateJobsStatus(js *sinks.JobSink) error {
	jobs, err := r.jobLister.Jobs(js.GetNamespace()).List(labels.SelectorFromSet(map[string]string{sinksc.JobSinkNameLabel: js.GetName()}))
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		// Reverse order, from the latest
		return jobs[i].Status.StartTime.Time.After(jobs[j].Status.StartTime.Time)
	})

	for _, job := range jobs {

		if len(js.Status.FailedJobs) <= maxFailedJobs {
			for _, c := range job.Status.Conditions {
				if c.Type == batchv1.JobFailed {
					js.Status.FailedJobs = append(js.Status.FailedJobs, job.Status)
				}
			}
		}
	}

	return nil
}

func (r *Reconciler) getCaCerts() (*string, error) {
	// Getting the secret called "imc-dispatcher-tls" from system namespace
	secret, err := r.secretLister.Secrets(r.systemNamespace).Get(eventingtls.JobSinkDispatcherServerTLSSecretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get CA certs from %s/%s: %w", r.systemNamespace, eventingtls.JobSinkDispatcherServerTLSSecretName, err)
	}
	caCerts, ok := secret.Data[eventingtls.SecretCACert]
	if !ok {
		return nil, nil
	}
	return pointer.String(string(caCerts)), nil
}

func (r *Reconciler) reconcileAddress(ctx context.Context, js *sinks.JobSink) error {

	featureFlags := feature.FromContext(ctx)
	if featureFlags.IsPermissiveTransportEncryption() {
		caCerts, err := r.getCaCerts()
		if err != nil {
			return err
		}

		httpAddress := r.httpAddress(js)
		httpsAddress := r.httpsAddress(caCerts, js)
		// Permissive mode:
		// - status.address http address with host-based routing
		// - status.addresses:
		//   - https address with path-based routing
		//   - http address with host-based routing
		js.Status.Addresses = []duckv1.Addressable{httpsAddress, httpAddress}
		js.Status.Address = &httpAddress
	} else if featureFlags.IsStrictTransportEncryption() {
		// Strict mode: (only https addresses)
		// - status.address https address with path-based routing
		// - status.addresses:
		//   - https address with path-based routing
		caCerts, err := r.getCaCerts()
		if err != nil {
			return err
		}

		httpsAddress := r.httpsAddress(caCerts, js)
		js.Status.Addresses = []duckv1.Addressable{httpsAddress}
		js.Status.Address = &httpsAddress
	} else {
		httpAddress := r.httpAddress(js)
		js.Status.Address = &httpAddress
	}

	js.GetConditionSet().Manage(js.GetStatus()).MarkTrue(sinks.JobSinkConditionAddressable)

	return nil
}

func (r *Reconciler) httpAddress(js *sinks.JobSink) duckv1.Addressable {
	// http address uses host-based routing
	httpAddress := duckv1.Addressable{
		Name: pointer.String("http"),
		URL: &apis.URL{
			Scheme: "http",
			Host:   network.GetServiceHostname("job-sink-dispatcher", r.systemNamespace),
			Path:   fmt.Sprintf("/%s/%s", js.GetNamespace(), js.GetName()),
		},
	}
	return httpAddress
}

func (r *Reconciler) httpsAddress(certs *string, js *sinks.JobSink) duckv1.Addressable {
	addr := r.httpAddress(js)
	addr.URL.Scheme = "https"
	addr.CACerts = certs
	return addr
}
