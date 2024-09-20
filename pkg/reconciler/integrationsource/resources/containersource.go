package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/kmeta"
	"strconv"
)

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

// Function to set env vars for spec objects
func makeEnv(source *v1alpha1.IntegrationSource) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	if source.Spec.Timer != nil {
		envVars = append(envVars, []corev1.EnvVar{
			makeBasicEnvVar("CAMEL_KAMELET_TIMER_SOURCE_PERIOD", strconv.Itoa(source.Spec.Timer.Period)),
			makeBasicEnvVar("CAMEL_KAMELET_TIMER_SOURCE_MESSAGE", source.Spec.Timer.Message),
			makeBasicEnvVar("CAMEL_KAMELET_TIMER_SOURCE_CONTENTTYPE", source.Spec.Timer.ContentType),
		}...)
		if source.Spec.Timer.RepeatCount > 0 {
			envVars = append(envVars, makeBasicEnvVar("CAMEL_KAMELET_TIMER_SOURCE_REPEATCOUNT", strconv.Itoa(source.Spec.Timer.RepeatCount)))
		}
		return envVars
	}

	// Handle secret name only if AWS is configured
	var secretName string
	if source.Spec.Aws != nil && source.Spec.Aws.Auth != nil && source.Spec.Aws.Auth.Secret != nil && source.Spec.Aws.Auth.Secret.Ref != nil {
		secretName = source.Spec.Aws.Auth.Secret.Ref.Name
	}

	if source.Spec.Aws != nil && source.Spec.Aws.S3 != nil {
		envVars = append(envVars, []corev1.EnvVar{
			makeSecretEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_ACCESSKEY", "aws.s3.accessKey", secretName),
			makeSecretEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_SECRETKEY", "aws.s3.secretKey", secretName),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_BUCKETNAMEORARN", source.Spec.Aws.S3.BucketNameOrArn),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_REGION", source.Spec.Aws.S3.Region),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_DELETEAFTERREAD", strconv.FormatBool(source.Spec.Aws.S3.DeleteAfterRead)),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_MOVEAFTERREAD", strconv.FormatBool(source.Spec.Aws.S3.MoveAfterRead)),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_DESTINATIONBUCKET", source.Spec.Aws.S3.DestinationBucket),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_PREFIX", source.Spec.Aws.S3.Prefix),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_IGNOREBODY", strconv.FormatBool(source.Spec.Aws.S3.IgnoreBody)),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_DELAY", strconv.Itoa(source.Spec.Aws.S3.Delay)),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_MAXMESSAGESPERPOLL", strconv.Itoa(source.Spec.Aws.S3.MaxMessagesPerPoll)),
		}...)
		return envVars
	}

	if source.Spec.Aws != nil && source.Spec.Aws.SQS != nil {
		envVars = append(envVars, []corev1.EnvVar{
			makeSecretEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_ACCESSKEY", "aws.s3.accessKey", secretName),
			makeSecretEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_SECRETKEY", "aws.s3.secretKey", secretName),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_QUEUENAMEORARN", source.Spec.Aws.SQS.QueueNameOrArn),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_REGION", source.Spec.Aws.SQS.Region),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_DELETEAFTERREAD", strconv.FormatBool(source.Spec.Aws.SQS.DeleteAfterRead)),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_AUTOCREATEQUEUE", strconv.FormatBool(source.Spec.Aws.SQS.AutoCreateQueue)),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_AMAZONAWSHOST", source.Spec.Aws.SQS.AmazonAWSHost),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_PROTOCOL", source.Spec.Aws.SQS.Protocol),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_QUEUEURL", source.Spec.Aws.SQS.QueueURL),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_GREEDY", strconv.FormatBool(source.Spec.Aws.SQS.Greedy)),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_DELAY", strconv.Itoa(source.Spec.Aws.SQS.Delay)),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_MAXMESSAGESPERPOLL", strconv.Itoa(source.Spec.Aws.SQS.MaxMessagesPerPoll)),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_WAITTIMESECONDS", strconv.Itoa(source.Spec.Aws.SQS.WaitTimeSeconds)),
			makeBasicEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_VISIBILITYTIMEOUT", strconv.Itoa(source.Spec.Aws.SQS.VisibilityTimeout)),
		}...)
		return envVars
	}

	return envVars
}

func selectImage(source *v1alpha1.IntegrationSource) string {
	if source.Spec.Timer != nil {
		return "quay.io/openshift-knative/kn-connector-timer-source:1.0-SNAPSHOT"
	}
	if source.Spec.Aws != nil {
		if source.Spec.Aws.S3 != nil {
			//source-timer
			//			return "quay.io/openshift-knative/kn-connector-source-timer:1.0.1-SNAPSHOT"
			return "quay.io/openshift-knative/kn-connector-aws-s3-source:1.0-SNAPSHOT"
		}
		if source.Spec.Aws.SQS != nil {
			return "quay.io/openshift-knative/kn-connector-aws-sqs-source:1.0-SNAPSHOT"
		}
	}
	return ""
}

func makeBasicEnvVar(name, value string) corev1.EnvVar {
	return corev1.EnvVar{
		Name:  name,
		Value: value,
	}
}

func makeSecretEnvVar(name, key, secretName string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				Key: key,
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
			},
		},
	}
}
