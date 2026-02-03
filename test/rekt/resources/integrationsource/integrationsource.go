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

package integrationsource

import (
	"context"
	"embed"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/test/rekt/resources/awsconfig"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed integrationsource.yaml
var yaml embed.FS

type SourceType string

const (
	SourceTypeTimer      SourceType = "dev.knative.eventing.timer"
	SourceTypeS3         SourceType = "dev.knative.eventing.aws-s3"
	SourceTypeSQS        SourceType = "dev.knative.eventing.aws-sqs"
	SourceTypeDDbStreams SourceType = "dev.knative.eventing.aws-ddb-streams"
)

// Environment variable names for source-specific AWS resources
const (
	EnvS3SourceArn     = "AWS_S3_SOURCE_ARN"
	EnvSQSSourceArn    = "AWS_SQS_SOURCE_ARN"
	EnvDDBStreamsTable = "AWS_DDB_STREAMS_TABLE"
)

// Default values for source-specific AWS resources
const (
	DefaultS3SourceArn     = "arn:aws:s3:::eventing-e2e-source"
	DefaultSQSSourceArn    = "arn:aws:sqs:us-west-1::eventing-e2e-sqs-source"
	DefaultDDBStreamsTable = "eventing-e2e-source"
)

// IntegrationSourceConfig holds source-specific AWS resource identifiers
type IntegrationSourceConfig struct {
	S3Arn           string
	SQSArn          string
	DDBStreamsTable string
}

type integrationSourceConfigKey struct{}

// WithAWSIntegrationSourceConfig loads source-specific config from environment into context
func WithAWSIntegrationSourceConfig(ctx context.Context, env environment.Environment) (context.Context, error) {
	cfg := &IntegrationSourceConfig{
		S3Arn:           getEnvOrDefault(EnvS3SourceArn, DefaultS3SourceArn),
		SQSArn:          getEnvOrDefault(EnvSQSSourceArn, DefaultSQSSourceArn),
		DDBStreamsTable: getEnvOrDefault(EnvDDBStreamsTable, DefaultDDBStreamsTable),
	}
	return context.WithValue(ctx, integrationSourceConfigKey{}, cfg), nil
}

// IntegrationSourceConfigFromContext retrieves source config from context
func IntegrationSourceConfigFromContext(ctx context.Context) *IntegrationSourceConfig {
	if cfg, ok := ctx.Value(integrationSourceConfigKey{}).(*IntegrationSourceConfig); ok {
		return cfg
	}
	panic("no IntegrationSourceConfig found in context")
}

func getEnvOrDefault(envVar, defaultValue string) string {
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return defaultValue
}

func Gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sources.knative.dev", Version: "v1alpha1", Resource: "integrationsources"}
}

// IsReady tests to see if a ContainerSource becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(Gvr(), name, timing...)
}

// Install will create a ContainerSource resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
	}
	for _, fn := range opts {
		fn(cfg)
	}

	return func(ctx context.Context, t feature.T) {
		if ic := environment.GetIstioConfig(ctx); ic.Enabled {
			manifest.WithIstioPodAnnotations(cfg)
		}

		//if err := registerImage(ctx); err != nil {
		//	t.Fatal(err)
		//}
		if _, err := manifest.InstallYamlFS(ctx, yaml, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// InstallByType installs an IntegrationSource based on the source type.
// For AWS sources (S3/SQS/DynamoDB Streams), it reads AWS credentials from context and environment,
// creates a Kubernetes secret, and configures the source appropriately.
func InstallByType(name string, sourceType SourceType, sinkOpts ...manifest.CfgFn) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		switch sourceType {
		case SourceTypeS3:
			awsCfg := awsconfig.AWSConfigFromContext(ctx)
			sourceCfg := IntegrationSourceConfigFromContext(ctx)
			secretName := feature.MakeRandomK8sName("aws-credentials")
			awsconfig.InstallAWSSecret(ctx, t, secretName)
			opts := append([]manifest.CfgFn{WithS3Source(sourceCfg.S3Arn, awsCfg.Region, secretName)}, sinkOpts...)
			Install(name, opts...)(ctx, t)

		case SourceTypeSQS:
			awsCfg := awsconfig.AWSConfigFromContext(ctx)
			sourceCfg := IntegrationSourceConfigFromContext(ctx)
			secretName := feature.MakeRandomK8sName("aws-credentials")
			awsconfig.InstallAWSSecret(ctx, t, secretName)
			opts := append([]manifest.CfgFn{WithSQSSource(sourceCfg.SQSArn, awsCfg.Region, secretName)}, sinkOpts...)
			Install(name, opts...)(ctx, t)

		case SourceTypeDDbStreams:
			awsCfg := awsconfig.AWSConfigFromContext(ctx)
			sourceCfg := IntegrationSourceConfigFromContext(ctx)
			secretName := feature.MakeRandomK8sName("aws-credentials")
			awsconfig.InstallAWSSecret(ctx, t, secretName)
			opts := append([]manifest.CfgFn{WithDynamoDBStreamsSource(sourceCfg.DDBStreamsTable, awsCfg.Region, secretName)}, sinkOpts...)
			Install(name, opts...)(ctx, t)

		case SourceTypeTimer:
			opts := append([]manifest.CfgFn{WithTimerSource()}, sinkOpts...)
			Install(name, opts...)(ctx, t)
		}
	}
}

func WithSink(d *duckv1.Destination) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["sink"]; !set {
			cfg["sink"] = map[string]interface{}{}
		}
		sink := cfg["sink"].(map[string]interface{})

		ref := d.Ref
		uri := d.URI

		if d.CACerts != nil {
			// This is a multi-line string and should be indented accordingly.
			// Replace "new line" with "new line + spaces".
			sink["CACerts"] = strings.ReplaceAll(*d.CACerts, "\n", "\n      ")
		}

		if uri != nil {
			sink["uri"] = uri.String()
		}

		if d.Audience != nil {
			sink["audience"] = *d.Audience
		}

		if ref != nil {
			if _, set := sink["ref"]; !set {
				sink["ref"] = map[string]interface{}{}
			}
			sref := sink["ref"].(map[string]interface{})
			sref["apiVersion"] = ref.APIVersion
			sref["kind"] = ref.Kind
			sref["namespace"] = ref.Namespace
			sref["name"] = ref.Name
		}
	}
}

func WithTimerSource() manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["integrationSourceType"] = string(SourceTypeTimer)
	}
}

func WithS3Source(arn, region, secretName string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["integrationSourceType"] = string(SourceTypeS3)
		cfg["s3Arn"] = arn
		cfg["s3Region"] = region
		cfg["secretName"] = secretName
	}
}

func WithSQSSource(arn, region, secretName string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["integrationSourceType"] = string(SourceTypeSQS)
		cfg["sqsArn"] = arn
		cfg["sqsRegion"] = region
		cfg["secretName"] = secretName
	}
}

func WithDynamoDBStreamsSource(tableName, region, secretName string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["integrationSourceType"] = string(SourceTypeDDbStreams)
		cfg["ddbStreamsTable"] = tableName
		cfg["ddbStreamsRegion"] = region
		cfg["secretName"] = secretName
	}
}

// TriggerEventByType triggers a message based on the source type.
// For S3 sources, uploads a file to S3 bucket.
// For SQS sources, sends a message to SQS queue.
// For DynamoDB Streams sources, puts an item to DynamoDB table.
// For Timer sources, does nothing (timer triggers automatically).
func TriggerEventByType(sourceType SourceType) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		switch sourceType {
		case SourceTypeS3:
			sourceCfg := IntegrationSourceConfigFromContext(ctx)
			awsconfig.UploadFileToS3(ctx, t, sourceCfg.S3Arn, "message.json", `{"message": "Hello from AWS S3!"}`)
		case SourceTypeSQS:
			sourceCfg := IntegrationSourceConfigFromContext(ctx)
			awsconfig.SendSQSMessage(ctx, t, sourceCfg.SQSArn, `{"message": "Hello from AWS SQS!"}`)
		case SourceTypeDDbStreams:
			sourceCfg := IntegrationSourceConfigFromContext(ctx)
			awsconfig.PutDynamoDBItem(ctx, t, sourceCfg.DDBStreamsTable, "Hello from AWS DynamoDB Streams!")
		case SourceTypeTimer:
			// Timer source triggers automatically, no action needed
		}
	}
}

// CleanupS3 returns a StepFn that cleans up the S3 source bucket.
func CleanupS3() feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		cfg := IntegrationSourceConfigFromContext(ctx)
		awsconfig.CleanupS3Bucket(ctx, t, cfg.S3Arn)
	}
}

// CleanupSQS returns a StepFn that cleans up the SQS source queue.
func CleanupSQS() feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		cfg := IntegrationSourceConfigFromContext(ctx)
		// Extract queue name from ARN (arn:aws:sqs:region:account:queue-name)
		parts := strings.Split(cfg.SQSArn, ":")
		queueName := parts[len(parts)-1]
		awsconfig.CleanupSQSQueue(ctx, t, queueName)
	}
}

// CleanupDynamoDB returns a StepFn that cleans up the DynamoDB source table.
func CleanupDynamoDB() feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		cfg := IntegrationSourceConfigFromContext(ctx)
		awsconfig.CleanupDynamoDBTable(ctx, t, cfg.DDBStreamsTable)
	}
}
