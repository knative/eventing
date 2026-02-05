package integrationsink

import (
	"context"
	"embed"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/test/rekt/resources/awsconfig"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed integrationsink.yaml
var yamlEmbed embed.FS

type SinkType string

const (
	SinkTypeLog SinkType = "log"
	SinkTypeS3  SinkType = "dev.knative.eventing.aws-s3"
	SinkTypeSQS SinkType = "dev.knative.eventing.aws-sqs"
	SinkTypeSNS SinkType = "dev.knative.eventing.aws-sns"
)

// Environment variable names for sink-specific AWS resources
const (
	EnvS3SinkArn                = "AWS_S3_SINK_ARN"
	EnvSQSQueueName             = "AWS_SQS_QUEUE_NAME"
	EnvSNSTopicName             = "AWS_SNS_TOPIC_NAME"
	EnvSNSVerificationQueueName = "AWS_SNS_VERIFICATION_QUEUE_NAME"
)

// Default values for sink-specific AWS resources
const (
	DefaultS3SinkArn                = "arn:aws:s3:::eventing-e2e-sink"
	DefaultSQSQueueName             = "eventing-e2e-sqs-sink"
	DefaultSNSTopicName             = "eventing-e2e-sns-sink"
	DefaultSNSVerificationQueueName = "eventing-e2e-sns-verification"
)

// IntegrationSinkConfig holds sink-specific AWS resource identifiers
type IntegrationSinkConfig struct {
	S3Arn                    string
	SQSQueueName             string
	SNSTopicName             string
	SNSVerificationQueueName string
}

type integrationSinkConfigKey struct{}

// WithAWSIntegrationSinkConfig loads sink-specific config from environment into context
func WithAWSIntegrationSinkConfig(ctx context.Context, env environment.Environment) (context.Context, error) {
	cfg := &IntegrationSinkConfig{
		S3Arn:                    getEnvOrDefault(EnvS3SinkArn, DefaultS3SinkArn),
		SQSQueueName:             getEnvOrDefault(EnvSQSQueueName, DefaultSQSQueueName),
		SNSTopicName:             getEnvOrDefault(EnvSNSTopicName, DefaultSNSTopicName),
		SNSVerificationQueueName: getEnvOrDefault(EnvSNSVerificationQueueName, DefaultSNSVerificationQueueName),
	}
	return context.WithValue(ctx, integrationSinkConfigKey{}, cfg), nil
}

// IntegrationSinkConfigFromContext retrieves sink config from context
func IntegrationSinkConfigFromContext(ctx context.Context) *IntegrationSinkConfig {
	if cfg, ok := ctx.Value(integrationSinkConfigKey{}).(*IntegrationSinkConfig); ok {
		return cfg
	}
	panic("no IntegrationSinkConfig found in context")
}

func getEnvOrDefault(envVar, defaultValue string) string {
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return defaultValue
}

func GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sinks.knative.dev", Version: "v1alpha1", Resource: "integrationsinks"}
}

// WithAnnotations adds annotations to the IntegrationSink.
func WithAnnotations(annotations map[string]any) manifest.CfgFn {
	return func(cfg map[string]any) {
		if annotations != nil {
			cfg["annotations"] = annotations
		}
	}
}

// Install will create a resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {

	return func(ctx context.Context, t feature.T) {
		cfg := map[string]any{
			"name":                           name,
			"namespace":                      environment.FromContext(ctx).Namespace(),
			"image":                          eventshub.ImageFromContext(ctx),
			eventshub.ConfigLoggingEnv:       knative.LoggingConfigFromContext(ctx),
			eventshub.ConfigObservabilityEnv: knative.ObservabilityConfigFromContext(ctx),
		}
		for _, fn := range opts {
			fn(cfg)
		}

		if _, err := manifest.InstallYamlFS(ctx, yamlEmbed, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// InstallByType installs an IntegrationSink based on the sink type.
// For AWS sinks (S3/SQS/SNS), it reads AWS credentials from context and environment,
// creates a Kubernetes secret, and configures the sink appropriately.
func InstallByType(name string, sinkType SinkType) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		switch sinkType {
		case SinkTypeS3:
			awsCfg := awsconfig.AWSConfigFromContext(ctx)
			sinkCfg := IntegrationSinkConfigFromContext(ctx)
			secretName := feature.MakeRandomK8sName("aws-credentials")
			awsconfig.InstallAWSSecret(ctx, t, secretName)
			Install(name, WithS3Sink(sinkCfg.S3Arn, awsCfg.Region, secretName))(ctx, t)

		case SinkTypeSQS:
			awsCfg := awsconfig.AWSConfigFromContext(ctx)
			sinkCfg := IntegrationSinkConfigFromContext(ctx)
			secretName := feature.MakeRandomK8sName("aws-credentials")
			awsconfig.InstallAWSSecret(ctx, t, secretName)
			Install(name, WithSQSSink(sinkCfg.SQSQueueName, awsCfg.Region, secretName))(ctx, t)

		case SinkTypeSNS:
			awsCfg := awsconfig.AWSConfigFromContext(ctx)
			sinkCfg := IntegrationSinkConfigFromContext(ctx)
			secretName := feature.MakeRandomK8sName("aws-credentials")
			awsconfig.InstallAWSSecret(ctx, t, secretName)
			Install(name, WithSNSSink(sinkCfg.SNSTopicName, awsCfg.Region, secretName))(ctx, t)

		case SinkTypeLog:
			Install(name, WithLogSink())(ctx, t)
		}
	}
}

// IsReady tests to see if a IntegrationSink becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(GVR(), name, timing...)
}

// IsNotReady tests to see if a IntegrationSink becomes NotReady within the time given.
func IsNotReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsNotReady(GVR(), name, timing...)
}

// IsAddressable tests to see if a IntegrationSink becomes addressable within the  time
// given.
func IsAddressable(name string, timings ...time.Duration) feature.StepFn {
	return k8s.IsAddressable(GVR(), name, timings...)
}

func GoesReadySimple(name string) *feature.Feature {
	f := feature.NewFeature()

	f.Setup("install integration sink", Install(name))
	f.Setup("integrationsink is ready", IsReady(name))
	f.Setup("integrationsink is addressable", IsAddressable(name))

	return f
}

func WithLogSink() manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["integrationSinkType"] = string(SinkTypeLog)
	}
}

func WithS3Sink(arn, region, secretName string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["integrationSinkType"] = string(SinkTypeS3)
		cfg["s3Arn"] = arn
		cfg["s3Region"] = region
		cfg["secretName"] = secretName
	}
}

func WithSQSSink(arn, region, secretName string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["integrationSinkType"] = string(SinkTypeSQS)
		cfg["sqsArn"] = arn
		cfg["sqsRegion"] = region
		cfg["secretName"] = secretName
	}
}

func WithSNSSink(arn, region, secretName string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["integrationSinkType"] = string(SinkTypeSNS)
		cfg["snsArn"] = arn
		cfg["snsRegion"] = region
		cfg["secretName"] = secretName
	}
}

// CleanupS3 returns a StepFn that cleans up the S3 bucket.
func CleanupS3() feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		cfg := IntegrationSinkConfigFromContext(ctx)
		awsconfig.CleanupS3Bucket(ctx, t, cfg.S3Arn)
	}
}

// AssertS3ObjectCount returns a StepFn that verifies S3 object count.
func AssertS3ObjectCount(expectedCount int) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		cfg := IntegrationSinkConfigFromContext(ctx)
		awsconfig.VerifyS3ObjectCount(ctx, t, cfg.S3Arn, expectedCount)
	}
}

// CleanupSQS returns a StepFn that cleans up the SQS queue.
func CleanupSQS() feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		cfg := IntegrationSinkConfigFromContext(ctx)
		awsconfig.CleanupSQSQueue(ctx, t, cfg.SQSQueueName)
	}
}

// AssertSQSMessages returns a StepFn that verifies SQS messages.
func AssertSQSMessages(expectedContent string, expectedCount int) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		cfg := IntegrationSinkConfigFromContext(ctx)
		awsconfig.VerifySQSMessages(ctx, t, cfg.SQSQueueName, expectedContent, expectedCount)
	}
}

// CleanupSNSVerificationQueue returns a StepFn that cleans up the SNS verification queue.
func CleanupSNSVerificationQueue() feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		cfg := IntegrationSinkConfigFromContext(ctx)
		awsconfig.CleanupSQSQueue(ctx, t, cfg.SNSVerificationQueueName)
	}
}

// AssertSNSMessages returns a StepFn that verifies SNS messages in the verification queue.
func AssertSNSMessages(expectedContent string, expectedCount int) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		cfg := IntegrationSinkConfigFromContext(ctx)
		awsconfig.VerifySNSMessages(ctx, t, cfg.SNSVerificationQueueName, expectedContent, expectedCount)
	}
}
