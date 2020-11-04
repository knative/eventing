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

	"github.com/wavesoftware/go-ensure"
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

const fetcherName = "wathola-fetcher"

func (p *prober) Verify(ctx context.Context) ([]error, int) {
	report := p.fetchReport(ctx)
	p.log.Infof("Fetched receiver report. Events propagated: %v. "+
		"State: %v", report.Events, report.State)
	if report.State == "active" {
		panic(errors.New("report fetched to early, receiver is in active state"))
	}
	errs := make([]error, 0)
	for _, t := range report.Thrown {
		errs = append(errs, errors.New(t))
	}
	return errs, report.Events
}

func (p *prober) Finish(ctx context.Context) {
	p.removeSender(ctx)
}

func (p *prober) fetchReport(ctx context.Context) *receiver.Report {
	exec := p.fetchExecution(ctx)
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
		logFunc("Fetcher: ", entry.Datetime, " ", entry.Message)
	}
}

func (p *prober) fetchExecution(ctx context.Context) *fetcher.Execution {
	ns := p.config.Namespace
	p.deployFetcher(ctx)
	defer p.deleteFetcher(ctx)
	pod := p.succeededJobPod(ctx, fetcherName, ns)
	bytes, err := pkgTest.PodLogs(ctx, p.client.Kube, pod.Name, fetcherName, ns)
	ensure.NoError(err)
	ex := &fetcher.Execution{
		Logs: []fetcher.LogEntry{},
		Report: &receiver.Report{
			State:  "failure",
			Events: 0,
			Thrown: []string{"Report wasn't fetched"},
		},
	}
	err = json.Unmarshal(bytes, ex)
	ensure.NoError(err)
	return ex
}

func (p *prober) deployFetcher(ctx context.Context) {
	p.log.Info("Deploying fetcher job: ", fetcherName)
	jobs := p.client.Kube.BatchV1().Jobs(p.config.Namespace)
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
						Image: pkgTest.ImagePath(fetcherName),
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
	_, err := jobs.Create(ctx, fetcherJob, metav1.CreateOptions{})
	ensure.NoError(err)
	p.log.Info("Waiting for fetcher job to succeed: ", fetcherName)
	err = waitForJobIsCompleted(ctx, p.client.Kube, fetcherName, p.config.Namespace)
	ensure.NoError(err)
}

func (p *prober) deleteFetcher(ctx context.Context) {
	ns := p.config.Namespace
	jobs := p.client.Kube.BatchV1().Jobs(ns)
	err := jobs.Delete(ctx, fetcherName, metav1.DeleteOptions{})
	ensure.NoError(err)
}

func (p *prober) succeededJobPod(ctx context.Context, jobName, namespace string) *corev1.Pod {
	pods := p.client.Kube.CoreV1().Pods(namespace)
	podList, err := pods.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprint("job-name=", jobName),
	})
	ensure.NoError(err)
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodSucceeded {
			return &pod
		}
	}
	return nil
}

func waitForJobIsCompleted(ctx context.Context, client kubernetes.Interface, jobName, namespace string) error {
	jobs := client.BatchV1().Jobs(namespace)

	span := logging.GetEmitableSpan(ctx, fmt.Sprint("waitForJobIsCompleted/", jobName))
	defer span.End()

	return wait.PollImmediate(time.Second, 2*time.Minute, func() (bool, error) {
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
