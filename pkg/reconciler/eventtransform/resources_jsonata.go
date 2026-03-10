/*
Copyright 2025 The Knative Authors

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

package eventtransform

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/certificates"
	"knative.dev/eventing/pkg/eventingtls"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/network"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"

	eventing "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
)

const (
	JsonataResourcesLabelKey      = "eventing.knative.dev/event-transform-jsonata"
	JsonataResourcesLabelValue    = "true"
	JsonataExpressionHashKey      = "eventing.knative.dev/event-transform-jsonata-expression-hash"
	JsonataCertificateRevisionKey = "eventing.knative.dev/event-transform-jsonata-certificate-revision"
	JsonataResourcesNameSuffix    = "-jsonata"
	JsonataExpressionDataKey      = "jsonata-expression"
	JsonataReplyExpressionDataKey = "jsonata-expression-reply"

	JsonataExpressionPath = "/etc/jsonata"

	JsonataResourcesSelector = JsonataResourcesLabelKey + "=" + JsonataResourcesLabelValue

	JsonataTLSVolumeName = "jsonata-tls-certs"
	JsonataTLSVolumePath = "/etc/jsonata-tls"
	JsonataTLSKeyPath    = JsonataTLSVolumePath + "/" + eventingtls.TLSKey
	JsonataTLSCertPath   = JsonataTLSVolumePath + "/" + eventingtls.TLSCrt

	JsonataAuthProxyRoleBindingName = "eventing-auth-proxy-eventtransform"
)

func jsonataExpressionConfigMap(_ context.Context, transform *eventing.EventTransform) corev1.ConfigMap {
	expression := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix),
			Namespace: transform.GetNamespace(),
			Labels:    jsonataLabels(transform),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(transform),
			},
		},
		Data: map[string]string{
			JsonataExpressionDataKey: transform.Spec.EventTransformations.Jsonata.Expression,
		},
	}

	if transform.Spec.Reply != nil && transform.Spec.Reply.Jsonata != nil {
		expression.Data[JsonataReplyExpressionDataKey] = transform.Spec.Reply.Jsonata.Expression
	}

	return expression
}

func jsonataDeployment(ctx context.Context, authProxyImage string, withCombinedTrustBundle bool, cw *reconcilersource.ConfigWatcher, expression *corev1.ConfigMap, certificate *cmv1.Certificate, transform *eventing.EventTransform, trustBundleConfigMapLister ...corev1listers.ConfigMapLister) appsv1.Deployment {
	image := os.Getenv("EVENT_TRANSFORM_JSONATA_IMAGE")
	if image == "" {
		panic("EVENT_TRANSFORM_JSONATA_IMAGE must be set")
	}

	d := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix),
			Namespace: transform.GetNamespace(),
			Labels:    jsonataLabels(transform),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(transform),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				// Keep these matchLabels fixed and stable across versions.
				MatchLabels: map[string]string{
					JsonataResourcesLabelKey: JsonataResourcesLabelValue,
					NameLabelKey:             transform.GetName(),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						JsonataResourcesLabelKey: JsonataResourcesLabelValue,
						NameLabelKey:             transform.GetName(),
					},
					Annotations: make(map[string]string),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "jsonata-event-transform",
							Image: image,
							Env: append(
								[]corev1.EnvVar{
									{
										Name:  "JSONATA_TRANSFORM_FILE_NAME",
										Value: filepath.Join(JsonataExpressionPath, JsonataExpressionDataKey),
									},
								},
								cw.ToEnvVars()...,
							),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      expression.GetName(),
									ReadOnly:  true,
									MountPath: JsonataExpressionPath,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.Bool(false),
								RunAsNonRoot:             ptr.Bool(true),
								ReadOnlyRootFilesystem:   ptr.Bool(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt32(8080),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds:           5,
								TimeoutSeconds:                10,
								PeriodSeconds:                 5,
								SuccessThreshold:              1,
								FailureThreshold:              3,
								TerminationGracePeriodSeconds: ptr.Int64(120),
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/readyz",
										Port:   intstr.FromInt32(8080),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 5,
								TimeoutSeconds:      10,
								PeriodSeconds:       5,
								SuccessThreshold:    1,
								FailureThreshold:    10,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: expression.GetName(),
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: expression.GetName(),
									},
									Optional: ptr.Bool(false),
								},
							},
						},
					},
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									PodAffinityTerm: corev1.PodAffinityTerm{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												JsonataResourcesLabelKey: JsonataResourcesLabelValue,
												NameLabelKey:             transform.GetName(),
											},
										},
										TopologyKey: "topology.kubernetes.io/zone",
									},
									Weight: 100,
								},
								{
									PodAffinityTerm: corev1.PodAffinityTerm{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												JsonataResourcesLabelKey: JsonataResourcesLabelValue,
												NameLabelKey:             transform.GetName(),
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
									Weight: 90,
								},
							},
						},
					},
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 0,
					},
				},
			},
		},
	}

	// hashPayload is used to detect and roll out a new deployment when expressions change.
	hashPayload := transform.Spec.EventTransformations.Jsonata.Expression

	if transform.Spec.Reply != nil {
		if transform.Spec.Reply.Discard != nil {
			if *transform.Spec.Reply.Discard {
				d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "JSONATA_DISCARD_RESPONSE_BODY",
					Value: "true",
				})
			} else {
				d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "JSONATA_DISCARD_RESPONSE_BODY",
					Value: "false",
				})
			}
		} else {
			d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
				Name:  "JSONATA_DISCARD_RESPONSE_BODY",
				Value: "false",
			})
		}

		if transform.Spec.Reply.Jsonata != nil {
			d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
				Name:  "JSONATA_RESPONSE_TRANSFORM_FILE_NAME",
				Value: filepath.Join(JsonataExpressionPath, JsonataReplyExpressionDataKey),
			})
			hashPayload += transform.Spec.Reply.Jsonata.Expression
		}
	}

	// hashPayload annotation is used to detect and roll out a new deployment when expressions change.
	hash := sha256.Sum256([]byte(hashPayload))
	d.Spec.Template.Annotations[JsonataExpressionHashKey] = base64.StdEncoding.EncodeToString(hash[:])

	if certificate != nil && certificate.Status.Revision != nil {
		// certificate revision annotation is used to detect and roll out a new deployment when certificates change.
		d.Spec.Template.Annotations[JsonataCertificateRevisionKey] = fmt.Sprintf("%d", *certificate.Status.Revision)
	}

	if feature.FromContext(ctx).IsStrictTransportEncryption() {
		// Disable HTTP Server in 'strict' mode.
		d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  "DISABLE_HTTP_SERVER",
				Value: "true",
			},
		)

		// Switch probes to use HTTPS Scheme and Port.
		d.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Port = intstr.FromInt32(8443)
		d.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
		d.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt32(8443)
		d.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
	}

	if withCombinedTrustBundle {
		d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name: "NODE_EXTRA_CA_CERTS",
				Value: filepath.Join(
					eventingtls.TrustBundleMountPath,
					eventingtls.TrustBundleCombinedPemFile,
				),
			},
		)
	}

	if f := feature.FromContext(ctx); f.IsStrictTransportEncryption() || f.IsPermissiveTransportEncryption() {

		// Inject TLS Cert and Key file paths.
		d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  "HTTPS_CERT_PATH",
				Value: JsonataTLSCertPath,
			},
			corev1.EnvVar{
				Name:  "HTTPS_KEY_PATH",
				Value: JsonataTLSKeyPath,
			},
		)

		// Inject TLS Cert and Key secret volume.
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      JsonataTLSVolumeName,
				ReadOnly:  true,
				MountPath: JsonataTLSVolumePath,
			},
		)
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: JsonataTLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: jsonataCertificateSecretName(transform),
					Optional:   ptr.Bool(false),
				},
			},
		})
	}

	if feature.FromContext(ctx).IsOIDCAuthentication() && authProxyImage != "" {
		var authProxyVolumeMounts []corev1.VolumeMount
		if f := feature.FromContext(ctx); f.IsStrictTransportEncryption() || f.IsPermissiveTransportEncryption() {
			authProxyVolumeMounts = []corev1.VolumeMount{{
				Name:      JsonataTLSVolumeName,
				ReadOnly:  true,
				MountPath: JsonataTLSVolumePath,
			}}
		}

		d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, corev1.Container{
			Name:            "auth-proxy",
			Image:           authProxyImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports: []corev1.ContainerPort{
				{ContainerPort: 3128, Protocol: corev1.ProtocolTCP, Name: "http"},
				{ContainerPort: 3129, Protocol: corev1.ProtocolTCP, Name: "https"},
				{ContainerPort: 8090, Protocol: corev1.ProtocolTCP, Name: "health"},
			},
			Env:          jsonataAuthProxyEnv(ctx, transform),
			VolumeMounts: authProxyVolumeMounts,
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/readyz",
						Port:   intstr.FromInt32(8090),
						Scheme: corev1.URISchemeHTTP,
					},
				},
				InitialDelaySeconds: 5,
				PeriodSeconds:       10,
				SuccessThreshold:    1,
				FailureThreshold:    3,
				TimeoutSeconds:      10,
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/healthz",
						Port:   intstr.FromInt32(8090),
						Scheme: corev1.URISchemeHTTP,
					},
				},
				InitialDelaySeconds: 5,
				PeriodSeconds:       10,
				SuccessThreshold:    1,
				FailureThreshold:    3,
				TimeoutSeconds:      10,
			},
			StartupProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/readyz",
						Port:   intstr.FromInt32(8090),
						Scheme: corev1.URISchemeHTTP,
					},
				},
				InitialDelaySeconds: 5,
				PeriodSeconds:       10,
				SuccessThreshold:    1,
				FailureThreshold:    3,
				TimeoutSeconds:      10,
			},
		})

		// add trustbundles so auth-proxy's token verifier does not need the trustbundle configmap lister for oidc discovery
		if len(trustBundleConfigMapLister) > 0 && trustBundleConfigMapLister[0] != nil {
			podspec, err := eventingtls.AddTrustBundleVolumes(trustBundleConfigMapLister[0], &d, &d.Spec.Template.Spec)
			if err == nil && podspec != nil {
				d.Spec.Template.Spec = *podspec
			}
		}

		// don't expose ports on jsonata container, traffic should reach only auth-proxy
		d.Spec.Template.Spec.Containers[0].Ports = nil
	}

	return d
}

func jsonataAuthProxyEnv(ctx context.Context, transform *eventing.EventTransform) []corev1.EnvVar {
	serviceName := kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix)
	serviceHostname := network.GetServiceHostname(serviceName, transform.Namespace)

	var sinkURI string
	if f := feature.FromContext(ctx); f.IsStrictTransportEncryption() || f.IsPermissiveTransportEncryption() {
		sinkURI = fmt.Sprintf("https://%s", serviceHostname)
	} else {
		sinkURI = fmt.Sprintf("http://%s", serviceHostname)
	}

	envVars := []corev1.EnvVar{
		{Name: "TARGET_HTTP_PORT", Value: "8080"},
		{Name: "TARGET_HTTPS_PORT", Value: "8443"},
		{Name: "PROXY_HTTP_PORT", Value: "3128"},
		{Name: "PROXY_HTTPS_PORT", Value: "3129"},
		{Name: "PROXY_HEALTH_PORT", Value: "8090"},
		{Name: "SYSTEM_NAMESPACE", Value: system.Namespace()},
		{Name: "PARENT_API_VERSION", Value: eventing.SchemeGroupVersion.String()},
		{Name: "PARENT_KIND", Value: "EventTransform"},
		{Name: "PARENT_NAME", Value: transform.Name},
		{Name: "PARENT_NAMESPACE", Value: transform.Namespace},
		{Name: "SINK_NAMESPACE", Value: transform.Namespace},
		{Name: "SINK_URI", Value: sinkURI},
	}

	if f := feature.FromContext(ctx); f.IsStrictTransportEncryption() || f.IsPermissiveTransportEncryption() {
		envVars = append(envVars,
			corev1.EnvVar{Name: "SINK_TLS_CERT_FILE", Value: JsonataTLSCertPath},
			corev1.EnvVar{Name: "SINK_TLS_KEY_FILE", Value: JsonataTLSKeyPath},
			corev1.EnvVar{Name: "SINK_TLS_CA_FILE", Value: JsonataTLSVolumePath + "/" + eventingtls.SecretCACert},
		)
	}

	if feature.FromContext(ctx).IsOIDCAuthentication() {
		audience := auth.GetAudience(eventing.SchemeGroupVersion.WithKind("EventTransform"), transform.ObjectMeta)
		envVars = append(envVars, corev1.EnvVar{Name: "SINK_AUDIENCE", Value: audience})
	}

	return envVars
}

func jsonataEventPolicyRoleBinding(transform *eventing.EventTransform) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-eventpolicy-reader", transform.Name),
			Namespace: transform.Namespace,
			Labels:    jsonataLabels(transform),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(transform),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "knative-eventing-eventpolicy-reader",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: transform.Namespace,
				Name:      "default",
			},
		},
	}
}

func jsonataAuthProxyRoleBinding(transform *eventing.EventTransform, allTransforms []*eventing.EventTransform) *rbacv1.RoleBinding {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JsonataAuthProxyRoleBindingName,
			Namespace: system.Namespace(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "knative-eventing-auth-proxy",
		},
	}

	serviceAccounts := map[types.NamespacedName]struct{}{}
	for _, t := range allTransforms {
		serviceAccounts[types.NamespacedName{Namespace: t.Namespace, Name: "default"}] = struct{}{}
	}
	serviceAccounts[types.NamespacedName{Namespace: transform.Namespace, Name: "default"}] = struct{}{}

	for sa := range serviceAccounts {
		rb.Subjects = append(rb.Subjects, rbacv1.Subject{
			Kind:      "ServiceAccount",
			Namespace: sa.Namespace,
			Name:      sa.Name,
		})
	}

	return rb
}

func jsonataService(ctx context.Context, transform *eventing.EventTransform) corev1.Service {
	s := corev1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:        kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix),
			Namespace:   transform.GetNamespace(),
			Labels:      jsonataLabels(transform),
			Annotations: make(map[string]string, 2),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(transform),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				JsonataResourcesLabelKey: JsonataResourcesLabelValue,
				NameLabelKey:             transform.GetName(),
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	httpTargetPort := int32(8080)
	httpsTargetPort := int32(8443)
	if feature.FromContext(ctx).IsOIDCAuthentication() {
		httpTargetPort = 3128
		httpsTargetPort = 3129
	}

	if f := feature.FromContext(ctx); f.IsStrictTransportEncryption() {
		s.Spec.Ports = []corev1.ServicePort{
			{
				Name:        "https",
				Protocol:    corev1.ProtocolTCP,
				AppProtocol: ptr.String("https"),
				Port:        443,
				TargetPort:  intstr.IntOrString{Type: intstr.Int, IntVal: httpsTargetPort},
			},
		}
	} else if f.IsPermissiveTransportEncryption() {
		s.Spec.Ports = []corev1.ServicePort{
			{
				Name:        "https",
				Protocol:    corev1.ProtocolTCP,
				AppProtocol: ptr.String("https"),
				Port:        443,
				TargetPort:  intstr.IntOrString{Type: intstr.Int, IntVal: httpsTargetPort},
			},
			{
				Name:        "http",
				Protocol:    corev1.ProtocolTCP,
				AppProtocol: ptr.String("http"),
				Port:        80,
				TargetPort:  intstr.IntOrString{Type: intstr.Int, IntVal: httpTargetPort},
			},
		}
	} else {
		s.Spec.Ports = []corev1.ServicePort{
			{
				Name:        "http",
				Protocol:    corev1.ProtocolTCP,
				AppProtocol: ptr.String("http"),
				Port:        80,
				TargetPort:  intstr.IntOrString{Type: intstr.Int, IntVal: httpTargetPort},
			},
		}
	}

	return s
}

func jsonataSinkBindingName(transform *eventing.EventTransform) string {
	return kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix)
}

func jsonataSinkBinding(_ context.Context, transform *eventing.EventTransform) sourcesv1.SinkBinding {
	name := jsonataSinkBindingName(transform)
	sb := sourcesv1.SinkBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: transform.GetNamespace(),
			Labels:    jsonataLabels(transform),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(transform),
			},
		},
		Spec: sourcesv1.SinkBindingSpec{
			SourceSpec: duckv1.SourceSpec{
				Sink: *transform.Spec.Sink.DeepCopy(),
			},
			BindingSpec: duckv1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Namespace:  transform.GetNamespace(),
					Name:       name,
				},
			},
		},
	}

	return sb
}

func jsonataLabels(transform *eventing.EventTransform) map[string]string {
	labels := make(map[string]string, 2)
	labels[JsonataResourcesLabelKey] = JsonataResourcesLabelValue
	labels[NameLabelKey] = transform.GetName()
	return labels
}

func jsonataCertificateSecretName(transform *eventing.EventTransform) string {
	cert := certificates.MakeCertificate(transform)
	return cert.Spec.SecretName
}

func jsonataCertificate(ctx context.Context, transform *eventing.EventTransform) *cmv1.Certificate {
	svc := jsonataService(ctx, transform)
	return certificates.MakeCertificate(transform,
		func(cert *cmv1.Certificate) {
			cert.Name = kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix)
		},
		certificates.WithDNSNames(
			network.GetServiceHostname(svc.Name, svc.Namespace),
			fmt.Sprintf("%s.%s.svc", svc.Name, svc.Namespace),
		),
	)
}
