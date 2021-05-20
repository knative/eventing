/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package prober

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"knative.dev/eventing/test/upgrade/prober/wathola/fetcher"
	"knative.dev/eventing/test/upgrade/prober/wathola/receiver"
	pkgTest "knative.dev/pkg/test"
	"knative.dev/pkg/test/logging"
)

const (
	fetcherName     = "wathola-fetcher"
	jobWaitInterval = time.Second
	jobWaitTimeout  = 5 * time.Minute
)

// Verify will verify prober state after finished has been sent.
func (p *prober) Verify() (eventErrs []error, eventsSent int) {
	report := p.fetchReport()
	availRate := 0.0
	if report.TotalRequests != 0 {
		availRate = float64(report.EventsSent*100) / float64(report.TotalRequests)
	}
	p.log.Infof("Fetched receiver report. Events propagated: %v. State: %v.",
		report.EventsSent, report.State)
	p.log.Infof("Availability: %.3f%%, Requests sent: %d.",
		availRate, report.TotalRequests)
	if report.State == "active" {
		p.client.T.Fatal("report fetched too early, receiver is in active state")
	}
	for _, t := range report.Thrown.Missing {
		eventErrs = append(eventErrs, errors.New(t))
	}
	for _, t := range report.Thrown.Unexpected {
		eventErrs = append(eventErrs, errors.New(t))
	}
	for _, t := range report.Thrown.Unavailable {
		eventErrs = append(eventErrs, errors.New(t))
	}
	for _, t := range report.Thrown.Duplicated {
		if p.config.OnDuplicate == Warn {
			p.log.Warn("Duplicate events: ", t)
		} else if p.config.OnDuplicate == Error {
			eventErrs = append(eventErrs, errors.New(t))
		}
	}
	return eventErrs, report.EventsSent
}

// Finish terminates sender which sends finished event.
func (p *prober) Finish() {
	p.removeSender()
}

func (p *prober) fetchReport() *receiver.Report {
	exec := p.fetchExecution()
	replayLogs(p.log, exec)
	return exec.Report
}

func replayLogs(log *zap.SugaredLogger, exec *fetcher.Execution) {
	for _, entry := range exec.Logs {
		logFunc := log.Error
		switch entry.Level {
		case "debug":
			logFunc = log.Debug
		case "info":
			logFunc = log.Info
		case "warning":
			logFunc = log.Warn
		}
		logFunc("Reply of fetcher log: ", entry.Datetime, " ", entry.Message)
	}
}

func (p *prober) fetchExecution() *fetcher.Execution {
	ns := p.client.Namespace
	job := p.deployFetcher()
	defer p.deleteFetcher()
	pod, err := p.findSucceededPod(job)
	p.ensureNoError(err)
	bytes, err := pkgTest.PodLogs(p.config.Ctx, p.client.Kube, pod.Name, fetcherName, ns)
	p.ensureNoError(err)
	ex := &fetcher.Execution{
		Logs: []fetcher.LogEntry{},
		Report: &receiver.Report{
			State:         "failure",
			EventsSent:    0,
			TotalRequests: 0,
			Thrown: receiver.Thrown{
				Unexpected:  []string{"Report wasn't fetched"},
				Missing:     []string{"Report wasn't fetched"},
				Duplicated:  []string{"Report wasn't fetched"},
				Unavailable: []string{"Report wasn't fetched"},
			},
		},
	}
	err = json.Unmarshal(bytes, ex)
	p.ensureNoError(err)
	return ex
}

func (p *prober) deployFetcher() *batchv1.Job {
	p.log.Info("Deploying fetcher job: ", fetcherName)
	jobs := p.client.Kube.BatchV1().Jobs(p.client.Namespace)
	var replicas int32 = 1
	fetcherJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fetcherName,
			Namespace: p.client.Namespace,
		},
		Spec: batchv1.JobSpec{
			Completions: &replicas,
			Parallelism: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": fetcherName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{{
						Name: p.config.ConfigMapName,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: p.config.ConfigMapName,
								},
							},
						},
					}},
					Containers: []corev1.Container{{
						Name:  fetcherName,
						Image: p.config.ImageResolver(fetcherName),
						VolumeMounts: []corev1.VolumeMount{{
							Name:      p.config.ConfigMapName,
							ReadOnly:  true,
							MountPath: p.config.ConfigMountPoint,
						}},
					}},
				},
			},
		},
	}
	created, err := jobs.Create(p.config.Ctx, fetcherJob, metav1.CreateOptions{})
	p.ensureNoError(err)
	p.log.Info("Waiting for fetcher job to succeed: ", fetcherName)
	err = waitForJobToComplete(p.config.Ctx, p.client.Kube, fetcherName, p.client.Namespace)
	p.ensureNoError(err)

	return created
}

func (p *prober) deleteFetcher() {
	ns := p.client.Namespace
	jobs := p.client.Kube.BatchV1().Jobs(ns)
	err := jobs.Delete(p.config.Ctx, fetcherName, metav1.DeleteOptions{})
	p.ensureNoError(err)
}

func (p *prober) findSucceededPod(job *batchv1.Job) (*corev1.Pod, error) {
	pods := p.client.Kube.CoreV1().Pods(job.Namespace)
	podList, err := pods.List(p.config.Ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprint("job-name=", job.Name),
	})
	p.ensureNoError(err)
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodSucceeded {
			return &pod, nil
		}
	}
	return nil, fmt.Errorf(
		"could't find succeeded pod for job: %s", job.Name,
	)
}

func waitForJobToComplete(ctx context.Context, client kubernetes.Interface, jobName, namespace string) error {
	jobs := client.BatchV1().Jobs(namespace)

	span := logging.GetEmitableSpan(ctx, fmt.Sprint("waitForJobToComplete/", jobName))
	defer span.End()

	return wait.PollImmediate(jobWaitInterval, jobWaitTimeout, func() (bool, error) {
		j, err := jobs.Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return true, err
		}

		return j.Status.Succeeded == *j.Spec.Completions, nil
	})
}

// Report represents a receiver JSON report
type Report struct {
	State  string   `json:"state"`
	Events int      `json:"events"`
	Thrown []string `json:"thrown"`
}
