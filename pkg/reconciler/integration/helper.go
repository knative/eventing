package integration

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"strconv"
	"strings"
)

const prefix = "CAMEL_KAMELET_"

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

func MakeBaseVariableName(variable, suffix string) string {
	var builder strings.Builder
	// Calculate the total length to allocate memory efficiently
	totalLength := len(prefix) + len(variable) + len(suffix)
	builder.Grow(totalLength)
	builder.WriteString(prefix)
	builder.WriteString(variable)
	builder.WriteString(suffix)
	return builder.String()
}
