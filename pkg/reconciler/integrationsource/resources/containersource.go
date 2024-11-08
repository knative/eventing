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
	"fmt"
	"reflect"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/kmeta"
)

const (
	awsAccessKey = "aws.accessKey"
	awsSecretKey = "aws.secretKey"
)

func NewContainerSource(source *v1alpha1.IntegrationSource) *sourcesv1.ContainerSource {
	return &sourcesv1.ContainerSource{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
			},
			Name:      ContainerSourceName(source),
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

func generateEnvVarsFromStruct(prefix string, s interface{}) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	// Use reflection to inspect the struct fields
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Handle embedded/anonymous structs recursively
		if fieldType.Anonymous && field.Kind() == reflect.Struct {
			// Recursively handle embedded structs with the same prefix
			envVars = append(envVars, generateEnvVarsFromStruct(prefix, field.Interface())...)
			continue
		}

		// Extract the JSON tag or fall back to the Go field name
		jsonTag := fieldType.Tag.Get("json")
		tagName := strings.Split(jsonTag, ",")[0]

		// fallback to Go field name if no JSON tag
		if tagName == "" || tagName == "-" {
			tagName = fieldType.Name
		}

		envVarName := fmt.Sprintf("%s_%s", prefix, strings.ToUpper(tagName))

		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				continue
			}
			field = field.Elem()
		}

		var value string
		switch field.Kind() {
		case reflect.Int, reflect.Int32, reflect.Int64:
			value = strconv.FormatInt(field.Int(), 10)
		case reflect.Bool:
			value = strconv.FormatBool(field.Bool())
		case reflect.String:
			value = field.String()
		default:
			// Skip unsupported types
			continue
		}

		// Skip zero/empty values
		if value == "" {
			continue
		}

		envVars = append(envVars, corev1.EnvVar{
			Name:  envVarName,
			Value: value,
		})
	}

	return envVars
}

// Function to create environment variables for Timer or AWS configurations dynamically
func makeEnv(source *v1alpha1.IntegrationSource) []corev1.EnvVar {
	var envVars = makeSSLEnvVar()

	// Timer environment variables
	if source.Spec.Timer != nil {
		envVars = append(envVars, generateEnvVarsFromStruct("CAMEL_KAMELET_TIMER_SOURCE", *source.Spec.Timer)...)
		return envVars
	}

	// Handle secret name only if AWS is configured
	var secretName string
	if source.Spec.Aws != nil && source.Spec.Aws.Auth != nil && source.Spec.Aws.Auth.Secret != nil && source.Spec.Aws.Auth.Secret.Ref != nil {
		secretName = source.Spec.Aws.Auth.Secret.Ref.Name
	}

	// AWS S3 environment variables
	if source.Spec.Aws != nil && source.Spec.Aws.S3 != nil {
		envVars = append(envVars, generateEnvVarsFromStruct("CAMEL_KAMELET_AWS_S3_SOURCE", *source.Spec.Aws.S3)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				makeSecretEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_ACCESSKEY", awsAccessKey, secretName),
				makeSecretEnvVar("CAMEL_KAMELET_AWS_S3_SOURCE_SECRETKEY", awsSecretKey, secretName),
			}...)
		}
		return envVars
	}

	// AWS SQS environment variables
	if source.Spec.Aws != nil && source.Spec.Aws.SQS != nil {
		envVars = append(envVars, generateEnvVarsFromStruct("CAMEL_KAMELET_AWS_SQS_SOURCE", *source.Spec.Aws.SQS)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				makeSecretEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_ACCESSKEY", awsAccessKey, secretName),
				makeSecretEnvVar("CAMEL_KAMELET_AWS_SQS_SOURCE_SECRETKEY", awsSecretKey, secretName),
			}...)
		}
		return envVars
	}

	// AWS DynamoDB Streams environment variables
	if source.Spec.Aws != nil && source.Spec.Aws.DDBStreams != nil {
		envVars = append(envVars, generateEnvVarsFromStruct("CAMEL_KAMELET_AWS_DDB_STREAMS_SOURCE", *source.Spec.Aws.DDBStreams)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				makeSecretEnvVar("CAMEL_KAMELET_AWS_DDB_STREAMS_SOURCE_ACCESSKEY", awsAccessKey, secretName),
				makeSecretEnvVar("CAMEL_KAMELET_AWS_DDB_STREAMS_SOURCE_SECRETKEY", awsSecretKey, secretName),
			}...)
		}
		return envVars
	}

	// If no valid configuration is found, return empty envVars
	return envVars
}

func makeSSLEnvVar() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "CAMEL_KNATIVE_CLIENT_SSL_ENABLED",
			Value: "true",
		},
		{
			Name:  "CAMEL_KNATIVE_CLIENT_SSL_CERT_PATH",
			Value: "/knative-custom-certs/knative-eventing-bundle.pem",
		},
	}
}

func selectImage(source *v1alpha1.IntegrationSource) string {
	if source.Spec.Timer != nil {
		return "gcr.io/knative-nightly/timer-source:latest"
	}
	if source.Spec.Aws != nil {
		if source.Spec.Aws.S3 != nil {
			return "gcr.io/knative-nightly/aws-s3-source:latest"
		}
		if source.Spec.Aws.SQS != nil {
			return "gcr.io/knative-nightly/aws-sqs-source:latest"
		}
		if source.Spec.Aws.DDBStreams != nil {
			return "gcr.io/knative-nightly/aws-ddb-streams-source:latest"
		}
	}
	return ""
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
