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

package integration

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GenerateEnvVarsFromStruct(prefix string, s interface{}) []corev1.EnvVar {
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
			envVars = append(envVars, GenerateEnvVarsFromStruct(prefix, field.Interface())...)
			continue
		}

		// First, check for the custom 'camel' tag
		envVarName := fieldType.Tag.Get("camel")
		if envVarName != "" {
			envVarName = fmt.Sprintf("%s_%s", prefix, envVarName)
		} else {
			// If 'camel' tag is not present, fall back to the 'json' tag or Go field name
			jsonTag := fieldType.Tag.Get("json")
			tagName := strings.Split(jsonTag, ",")[0]
			if tagName == "" || tagName == "-" {
				tagName = fieldType.Name
			}
			envVarName = fmt.Sprintf("%s_%s", prefix, strings.ToUpper(tagName))
		}

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

func MakeSecretEnvVar(name, key, secretName string) corev1.EnvVar {
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

func MakeSSLEnvVar() []corev1.EnvVar {
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

func Labels(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": name,
	}
}

func LivenessProbe(port int32, scheme corev1.URIScheme) *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold: 3,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/q/health/live",
				Port:   intstr.FromInt32(port),
				Scheme: scheme,
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		TimeoutSeconds:      10,
	}
}

func ReadinessProbe(port int32, scheme corev1.URIScheme) *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold: 3,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/q/health/ready",
				Port:   intstr.FromInt32(port),
				Scheme: scheme,
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		TimeoutSeconds:      10,
	}
}

func StartupProbe(port int32, scheme corev1.URIScheme) *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold: 3,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/q/health/started",
				Port:   intstr.FromInt32(port),
				Scheme: scheme,
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		TimeoutSeconds:      10,
	}
}
