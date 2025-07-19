/*
Copyright 2019 The Knative Authors

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
	"context"
	"crypto/md5" //nolint:gosec
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered"
	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"

	"github.com/cloudevents/sdk-go/v2/binding"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	configmap "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	k8sruntime "knative.dev/pkg/observability/runtime/k8s"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"

	"knative.dev/pkg/signals"

	cmdbroker "knative.dev/eventing/cmd/broker"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/apis/sinks"
	sinksv "knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/eventing/pkg/auth"
	eventpolicyinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy"
	"knative.dev/eventing/pkg/client/injection/informers/sinks/v1alpha1/jobsink"
	sinkslister "knative.dev/eventing/pkg/client/listers/sinks/v1alpha1"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/observability"
	o11yconfigmap "knative.dev/eventing/pkg/observability/configmap"
	"knative.dev/eventing/pkg/observability/otel"
	"knative.dev/eventing/pkg/utils"
)

const (
	component = "job_sink"
	ScopeName = "knative.dev/cmd/jobsink"
)

var (
	latencyBounds = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
)

func main() {

	ctx := signals.NewContext()

	cfg := injection.ParseAndGetRESTConfigOrDie()
	ctx = injection.WithConfig(ctx, cfg)
	ctx = filteredFactory.WithSelectors(ctx,
		eventingtls.TrustBundleLabelSelector,
	)

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)
	ctx = injection.WithConfig(ctx, cfg)

	loggingConfig, err := cmdbroker.GetLoggingConfig(ctx, system.Namespace(), logging.ConfigMapName())
	if err != nil {
		log.Fatal("Error loading/parsing logging configuration:", err)
	}
	sl, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, component)
	logger := sl.Desugar()
	defer flush(sl)

	pprof := k8sruntime.NewProfilingServer(sl.Named("pprof"))

	mp, tp := otel.SetupObservabilityOrDie(ctx, "jobsink", sl, pprof)

	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := mp.Shutdown(ctx); err != nil {
			sl.Errorw("Error flushing metrics", zap.Error(err))
		}

		if err := tp.Shutdown(ctx); err != nil {
			sl.Errorw("Error flushing traces", zap.Error(err))
		}
	}()

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeclient.Get(ctx), system.Namespace())
	// Watch the observability config map and dynamically update metrics exporter.
	configMapWatcher.Watch(o11yconfigmap.Name(), pprof.UpdateFromConfigMap)
	// Watch the observability config map and dynamically update request logs.
	configMapWatcher.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(sl, atomicLevel, component))

	logger.Info("Starting the JobSink Ingress")

	trustBundleConfigMapLister := configmapinformer.Get(ctx, eventingtls.TrustBundleLabelSelector).Lister().ConfigMaps(system.Namespace())
	var h *Handler

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"))
	featureStore.WatchConfigs(configMapWatcher)

	// Decorate contexts with the current state of the feature config.
	ctxFunc := func(ctx context.Context) context.Context {
		return logging.WithLogger(featureStore.ToContext(ctx), sl)
	}

	h = &Handler{
		k8s:          kubeclient.Get(ctx),
		lister:       jobsink.Get(ctx).Lister(),
		withContext:  ctxFunc,
		authVerifier: auth.NewVerifier(ctx, eventpolicyinformer.Get(ctx).Lister(), trustBundleConfigMapLister, configMapWatcher),
	}

	meter := mp.Meter(ScopeName)

	h.dispatchDuration, err = meter.Float64Histogram(
		"kn.eventing.dispatch.duration",
		metric.WithDescription("The duration to dispatch the event"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latencyBounds...),
	)
	if err != nil {
		logger.Fatal("failed to create dispatch metric", zap.Error(err))
	}

	handler := otel.NewHandler(h, "receive", mp, tp)

	tlsConfig, err := getServerTLSConfig(ctx)
	if err != nil {
		log.Fatal("Failed to get TLS config", err)
	}

	sm, err := eventingtls.NewServerManager(ctx,
		kncloudevents.NewHTTPEventReceiver(8080),
		kncloudevents.NewHTTPEventReceiver(8443,
			kncloudevents.WithTLSConfig(tlsConfig)),
		handler,
		configMapWatcher,
	)
	if err != nil {
		logger.Fatal("failed to start eventingtls server", zap.Error(err))
	}

	// configMapWatcher does not block, so start it first.
	logger.Info("Starting ConfigMap watcher")
	if err = configMapWatcher.Start(ctx.Done()); err != nil {
		logger.Fatal("Failed to start ConfigMap watcher", zap.Error(err))
	}

	// Start informers and wait for them to sync.
	logger.Info("Starting informers.")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Fatal("Failed to start informers", zap.Error(err))
	}

	// Start the servers
	logger.Info("Starting...")
	if err = sm.StartServers(ctx); err != nil {
		logger.Fatal("StartServers() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")
}

type Handler struct {
	k8s              kubernetes.Interface
	lister           sinkslister.JobSinkLister
	withContext      func(ctx context.Context) context.Context
	authVerifier     *auth.Verifier
	dispatchDuration metric.Float64Histogram
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r.Context())
	logger := logging.FromContext(ctx).Desugar()
	start := time.Now()

	ctx = observability.WithRequestLabels(ctx, r)

	defer func() {
		labeler, _ := otelhttp.LabelerFromContext(ctx)
		h.dispatchDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(labeler.Get()...))
	}()

	if r.Method == http.MethodGet {
		h.handleGet(ctx, w, r)
		return
	}

	if r.Method != http.MethodPost {
		logger.Info("Unexpected HTTP method", zap.String("method", r.Method))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	parts := strings.Split(strings.TrimSuffix(r.RequestURI, "/"), "/")
	if len(parts) != 3 {
		logger.Info("Malformed uri", zap.String("URI", r.RequestURI), zap.Any("parts", parts))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ref := types.NamespacedName{
		Namespace: parts[1],
		Name:      parts[2],
	}

	ctx = observability.WithSinkLabels(ctx, ref, "JobSink")

	js, err := h.lister.JobSinks(ref.Namespace).Get(ref.Name)
	if err != nil {
		logger.Warn("Failed to retrieve jobsink", zap.String("ref", ref.String()), zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx = observability.WithLowCardinalityMessagingLabels(ctx, fmt.Sprintf("job:%s-{jobIdentifierHash}.%s", js.Name, js.Namespace), "send")

	logger.Debug("Handling POST request", zap.String("URI", r.RequestURI))

	err = h.authVerifier.VerifyRequest(ctx, feature.FromContext(ctx), js.Status.Address.Audience, js.Namespace, js.Status.Policies, r, w)
	if err != nil {
		logger.Warn("Failed to verify AuthN and AuthZ.", zap.Error(err))
		return
	}

	message := cehttp.NewMessageFromHttpRequest(r)
	defer message.Finish(nil)

	event, err := binding.ToEvent(r.Context(), message)
	if err != nil {
		logger.Warn("failed to extract event from request", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := event.Validate(); err != nil {
		logger.Info("failed to validate event from request", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx = observability.WithMinimalEventLabels(ctx, event)

	jobName := toJobName(ref.Name, event.Source(), event.ID())
	logger.Debug("Getting job for event", zap.String("URI", r.RequestURI), zap.String("jobName", jobName))

	jobs, err := h.k8s.BatchV1().Jobs(js.GetNamespace()).List(r.Context(), metav1.ListOptions{
		LabelSelector: jobLabelSelector(ref, jobName),
		Limit:         1,
	})
	if err != nil {
		logger.Warn("Failed to retrieve job", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(jobs.Items) > 0 {
		w.Header().Add("Location", locationHeader(ref, event.Source(), event.ID()))
		w.WriteHeader(http.StatusAccepted)
		return
	}

	eventBytes, err := event.MarshalJSON()
	if err != nil {
		logger.Info("Failed to marshal event", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	js = js.DeepCopy() // Do not modify informer copy.
	js.SetDefaults(ctx)

	job := js.Spec.Job.DeepCopy()
	job.Name = jobName
	if job.Labels == nil {
		job.Labels = make(map[string]string, 4)
	}
	job.Labels[sinks.JobSinkIDLabel] = jobName
	job.Labels[sinks.JobSinkNameLabel] = ref.Name
	job.OwnerReferences = append(job.OwnerReferences, metav1.OwnerReference{
		APIVersion:         sinksv.SchemeGroupVersion.String(),
		Kind:               sinks.JobSinkResource.Resource,
		Name:               js.GetName(),
		UID:                js.GetUID(),
		Controller:         ptr.Bool(true),
		BlockOwnerDeletion: ptr.Bool(false),
	})
	var mountPathName string
	for i := range job.Spec.Template.Spec.Containers {
		found := false
		for j := range job.Spec.Template.Spec.Containers[i].VolumeMounts {
			if job.Spec.Template.Spec.Containers[i].VolumeMounts[j].Name == "jobsink-event" {
				found = true
				mountPathName = job.Spec.Template.Spec.Containers[i].VolumeMounts[j].MountPath
				break
			}
		}
		if !found {
			job.Spec.Template.Spec.Containers[i].VolumeMounts = append(job.Spec.Template.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
				Name:      "jobsink-event",
				ReadOnly:  true,
				MountPath: "/etc/jobsink-event",
			})
			mountPathName = "/etc/jobsink-event"
		}
		job.Spec.Template.Spec.Containers[i].Env = append(job.Spec.Template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  "K_EVENT_PATH",
			Value: mountPathName,
		})
	}

	found := false
	for i := range job.Spec.Template.Spec.Volumes {
		if job.Spec.Template.Spec.Volumes[i].Name == "jobsink-event" {
			found = true
			break
		}
	}
	if !found {
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "jobsink-event",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: jobName},
			},
		})
	}

	logger.Debug("Creating job for event",
		zap.String("URI", r.RequestURI),
		zap.String("jobName", jobName),
		zap.Any("job", job),
	)

	createdJob, err := h.k8s.BatchV1().Jobs(ref.Namespace).Create(r.Context(), job, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		logger.Warn("Failed to create job", zap.Error(err))

		w.Header().Add("Reason", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if apierrors.IsAlreadyExists(err) {
		logger.Debug("Job already exists", zap.String("URI", r.RequestURI), zap.String("jobName", jobName))
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ref.Namespace,
			Labels: map[string]string{
				sinks.JobSinkIDLabel:   jobName,
				sinks.JobSinkNameLabel: ref.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "batch/v1",
					Kind:               "Job",
					Name:               createdJob.Name,
					UID:                createdJob.UID,
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(false),
				},
			},
		},
		Immutable: ptr.Bool(true),
		Data:      map[string][]byte{"event": eventBytes},
		Type:      corev1.SecretTypeOpaque,
	}

	logger.Debug("Creating secret for event",
		zap.String("URI", r.RequestURI),
		zap.String("jobName", jobName),
		zap.Any("secret.metadata", secret.ObjectMeta),
	)

	_, err = h.k8s.CoreV1().Secrets(ref.Namespace).Create(r.Context(), secret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		logger.Warn("Failed to create secret", zap.Error(err))

		w.Header().Add("Reason", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if apierrors.IsAlreadyExists(err) {
		logger.Debug("Secret already exists", zap.String("URI", r.RequestURI), zap.String("jobName", jobName))
	}

	w.Header().Add("Location", locationHeader(ref, event.Source(), event.ID()))
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) handleGet(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(ctx)
	parts := strings.Split(strings.TrimSuffix(r.RequestURI, "/"), "/")
	if len(parts) != 9 {
		logger.Info("Malformed uri", zap.String("URI", r.RequestURI))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ref := types.NamespacedName{
		Namespace: parts[2],
		Name:      parts[4],
	}

	js, err := h.lister.JobSinks(ref.Namespace).Get(ref.Name)
	if err != nil {
		logger.Warn("Failed to retrieve jobsink", zap.String("ref", ref.String()), zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logger.Debug("Handling GET request", zap.String("URI", r.RequestURI))

	err = h.authVerifier.VerifyRequest(ctx, feature.FromContext(ctx), js.Status.Address.Audience, js.Namespace, js.Status.Policies, r, w)
	if err != nil {
		logger.Warn("Failed to verify AuthN and AuthZ.", zap.Error(err))
		return
	}

	eventSource := parts[6]
	eventID := parts[8]

	jobName := toJobName(ref.Name, eventSource, eventID)

	job, err := h.k8s.BatchV1().Jobs(ref.Namespace).Get(r.Context(), jobName, metav1.GetOptions{})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed {
			if c.Status == corev1.ConditionTrue {
				w.Header().Add("Reason", "Failed")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		if c.Type == batchv1.JobComplete {
			if c.Status == corev1.ConditionTrue {
				w.Header().Add("Reason", "Complete")
				w.WriteHeader(http.StatusOK)
				return
			}
		}
	}

	w.Header().Add("Location", locationHeader(ref, eventSource, eventID))
	w.WriteHeader(http.StatusAccepted)
}

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
}

func getServerTLSConfig(ctx context.Context) (*tls.Config, error) {
	secret := types.NamespacedName{
		Namespace: system.Namespace(),
		Name:      eventingtls.JobSinkDispatcherServerTLSSecretName,
	}

	serverTLSConfig := eventingtls.NewDefaultServerConfig()
	serverTLSConfig.GetCertificate = eventingtls.GetCertificateFromSecret(ctx, secretinformer.Get(ctx), kubeclient.Get(ctx), secret)
	return eventingtls.GetTLSServerConfig(serverTLSConfig)
}

func locationHeader(ref types.NamespacedName, source, id string) string {
	return fmt.Sprintf("/namespaces/%s/name/%s/sources/%s/ids/%s", ref.Namespace, ref.Name, source, id)
}

func jobLabelSelector(ref types.NamespacedName, id string) string {
	return fmt.Sprintf("%s=%s,%s=%s", sinks.JobSinkIDLabel, id, sinks.JobSinkNameLabel, ref.Name)
}

func toJobName(js string, source, id string) string {
	h := md5.Sum([]byte(source + id)) //nolint:gosec
	return kmeta.ChildName(js+"-", utils.ToDNS1123Subdomain(hex.EncodeToString(h[:])))
}
