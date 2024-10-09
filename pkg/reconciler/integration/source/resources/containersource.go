package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/reconciler/integration"
	"knative.dev/pkg/kmeta"
)

const componentSuffix = "_SOURCE"

func NewContainerSource(source *v1alpha1.IntegrationSource) *sourcesv1.ContainerSource {
	return &sourcesv1.ContainerSource{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
			},
			Name:      "kn-connector-source-" + source.Name,
			Namespace: source.Namespace,
		},
		Spec: sourcesv1.ContainerSourceSpec{

			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "source",
							Image:           selectImage(source),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env:             makeEnv(source),
						},
					},
				},
			},
			SourceSpec: source.Spec.SourceSpec,
		},
	}
}

// Function to create environment variables for Timer or AWS configurations dynamically
func makeEnv(source *v1alpha1.IntegrationSource) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	// Timer environment variables
	if source.Spec.Timer != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct(constructBaseSourceVariableName("TIMER"), *source.Spec.Timer)...)
		return envVars
	}

	// Handle secret name only if AWS is configured
	var secretName string
	if source.Spec.Aws != nil && source.Spec.Aws.Auth != nil && source.Spec.Aws.Auth.Secret != nil && source.Spec.Aws.Auth.Secret.Ref != nil {
		secretName = source.Spec.Aws.Auth.Secret.Ref.Name
	}

	// AWS S3 environment variables
	if source.Spec.Aws != nil && source.Spec.Aws.S3 != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct(constructBaseSourceVariableName("AWS_S3"), *source.Spec.Aws.S3)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_ACCESSKEY", "aws.s3.accessKey", secretName),
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_SECRETKEY", "aws.s3.secretKey", secretName),
			}...)
		}
		return envVars
	}

	// AWS SQS environment variables
	if source.Spec.Aws != nil && source.Spec.Aws.SQS != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct(constructBaseSourceVariableName("AWS_SQS"), *source.Spec.Aws.SQS)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_ACCESSKEY", "aws.s3.accessKey", secretName),
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_SECRETKEY", "aws.s3.secretKey", secretName),
			}...)
		}
		return envVars
	}

	// AWS DynamoDB Streams environment variables
	if source.Spec.Aws != nil && source.Spec.Aws.DDBStreams != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct(constructBaseSourceVariableName("AWS_DDB_STREAMS"), *source.Spec.Aws.DDBStreams)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_DDB_STREAMS_SOURCE_ACCESSKEY", "aws.s3.accessKey", secretName),
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_DDB_STREAMS_SOURCE_SECRETKEY", "aws.s3.secretKey", secretName),
			}...)
		}
		return envVars
	}

	// If no valid configuration is found, return empty envVars
	return envVars
}

func selectImage(source *v1alpha1.IntegrationSource) string {
	if source.Spec.Timer != nil {
		return "quay.io/openshift-knative/kn-connector-timer-source:1.0-SNAPSHOT"
	}
	if source.Spec.Aws != nil {
		if source.Spec.Aws.S3 != nil {
			return "quay.io/openshift-knative/kn-connector-aws-s3-source:1.0-SNAPSHOT"
		}
		if source.Spec.Aws.SQS != nil {
			return "quay.io/openshift-knative/kn-connector-aws-sqs-source:1.0-SNAPSHOT"
		}
		if source.Spec.Aws.DDBStreams != nil {
			return "quay.io/openshift-knative/kn-connector-aws-ddb-streams-source:1.0-SNAPSHOT"
		}
	}
	return ""
}

func constructBaseSourceVariableName(variable string) string {
	return integration.MakeBaseVariableName(variable, componentSuffix)
}
