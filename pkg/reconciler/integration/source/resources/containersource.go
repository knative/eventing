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

package resources

import (
	"encoding/json"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	commonv1a1 "knative.dev/eventing/pkg/apis/common/integration/v1alpha1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/reconciler/integration"
	"knative.dev/pkg/kmeta"
)

func NewContainerSource(source *v1alpha1.IntegrationSource, oidc bool) *sourcesv1.ContainerSource {
	podTemplateSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "source",
				Image:           selectImage(source),
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env:             makeEnv(source, oidc),
			},
		},
	}
	marshalledPodTemplateSpec, err := json.Marshal(podTemplateSpec)
	if err != nil {
		// TODO: better way to handle error
		panic(err)
	}
	err = json.Unmarshal(marshalledPodTemplateSpec, &source.Spec.Template.Spec)
	if err != nil {
		panic(err)
	}
	return &sourcesv1.ContainerSource{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
			},
			Name:      ContainerSourceName(source),
			Namespace: source.Namespace,
			Labels:    integration.Labels(source.Name),
		},
		Spec: sourcesv1.ContainerSourceSpec{
			Template:   *source.Spec.Template,
			SourceSpec: source.Spec.SourceSpec,
		},
	}
}

// Function to create environment variables for Timer or AWS configurations dynamically
func makeEnv(source *v1alpha1.IntegrationSource, oidc bool) []corev1.EnvVar {
	var envVars = integration.MakeSSLEnvVar()

	if oidc {
		envVars = append(envVars, []corev1.EnvVar{
			{
				Name:  "CAMEL_KNATIVE_CLIENT_OIDC_ENABLED",
				Value: "true",
			},
			{
				Name:  "CAMEL_KNATIVE_CLIENT_OIDC_TOKEN_PATH",
				Value: "file:///oidc/token",
			},
		}...)
	}

	// Timer environment variables
	if source.Spec.Timer != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct("CAMEL_KAMELET_TIMER_SOURCE", *source.Spec.Timer)...)
		return envVars
	}

	// Handle secret name only if AWS is configured
	var secretName string
	if source.Spec.Aws != nil && source.Spec.Aws.Auth != nil && source.Spec.Aws.Auth.Secret != nil && source.Spec.Aws.Auth.Secret.Ref != nil {
		secretName = source.Spec.Aws.Auth.Secret.Ref.Name
	}

	// AWS S3 environment variables
	if source.Spec.Aws != nil && source.Spec.Aws.S3 != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct("CAMEL_KAMELET_AWS_S3_SOURCE", *source.Spec.Aws.S3)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_ACCESSKEY", commonv1a1.AwsAccessKey, secretName),
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_SECRETKEY", commonv1a1.AwsSecretKey, secretName),
			}...)
		}
		return envVars
	}

	// AWS SQS environment variables
	if source.Spec.Aws != nil && source.Spec.Aws.SQS != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct("CAMEL_KAMELET_AWS_SQS_SOURCE", *source.Spec.Aws.SQS)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_ACCESSKEY", commonv1a1.AwsAccessKey, secretName),
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_SECRETKEY", commonv1a1.AwsSecretKey, secretName),
			}...)
		}
		return envVars
	}

	// AWS DynamoDB Streams environment variables
	if source.Spec.Aws != nil && source.Spec.Aws.DDBStreams != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct("CAMEL_KAMELET_AWS_DDB_STREAMS_SOURCE", *source.Spec.Aws.DDBStreams)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_DDB_STREAMS_SOURCE_ACCESSKEY", commonv1a1.AwsAccessKey, secretName),
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_DDB_STREAMS_SOURCE_SECRETKEY", commonv1a1.AwsSecretKey, secretName),
			}...)
		}
		return envVars
	}

	// If no valid configuration is found, return empty envVars
	return envVars
}

func selectImage(source *v1alpha1.IntegrationSource) string {
	// Injected in ./config/core/deployments/controller.yaml
	switch {
	case source.Spec.Timer != nil:
		return os.Getenv("INTEGRATION_SOURCE_TIMER_IMAGE")
	case source.Spec.Aws != nil && source.Spec.Aws.S3 != nil:
		return os.Getenv("INTEGRATION_SOURCE_AWS_S3_IMAGE")
	case source.Spec.Aws != nil && source.Spec.Aws.SQS != nil:
		return os.Getenv("INTEGRATION_SOURCE_AWS_SQS_IMAGE")
	case source.Spec.Aws != nil && source.Spec.Aws.DDBStreams != nil:
		return os.Getenv("INTEGRATION_SOURCE_AWS_DDB_STREAMS_IMAGE")
	default:
		return ""
	}
}
