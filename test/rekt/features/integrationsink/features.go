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

package integrationsink

import (
	"context"
	"time"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"knative.dev/eventing/test/rekt/features/awsconfig"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/integrationsink"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
)

// installSinkByType installs an integration sink based on the sink type.
// For AWS S3 sink, it reads AWS credentials from environment variables and creates a secret.
// For AWS SQS sink, it reads AWS credentials from environment variables and creates a secret.
// For AWS SNS sink, it reads AWS credentials from environment variables and creates a secret.
// Environment variables:
//   - AWS_ACCESS_KEY_ID (required for S3/SQS/SNS sink)
//   - AWS_SECRET_ACCESS_KEY (required for S3/SQS/SNS sink)
//   - AWS_REGION (optional, default: us-west-1)
//   - AWS_S3_SINK_ARN (optional for S3, default: arn:aws:s3:::eventing-e2e-sink)
//   - AWS_SQS_QUEUE_NAME (optional for SQS, default: eventing-e2e-sqs-sink)
//   - AWS_SNS_TOPIC_NAME (optional for SNS, default: arn:aws:sns:us-west-1:123456789012:eventing-e2e-sns-sink)
//   - AWS_SNS_VERIFICATION_QUEUE_NAME (optional for SNS verification, default: eventing-e2e-sns-verification)
func installSinkByType(ctx context.Context, t feature.T, sinkName string, sinkType integrationsink.SinkType) {
	switch sinkType {
	case integrationsink.SinkTypeS3:
		cfg := awsconfig.AWSConfigFromContext(ctx)
		arn := awsconfig.GetEnvOrDefault("AWS_S3_SINK_ARN", "arn:aws:s3:::eventing-e2e-sink")

		secretName := feature.MakeRandomK8sName("aws-credentials")
		awsconfig.InstallAWSSecret(ctx, t, secretName)
		integrationsink.Install(sinkName, integrationsink.WithS3Sink(arn, cfg.Region, secretName))(ctx, t)

	case integrationsink.SinkTypeSQS:
		cfg := awsconfig.AWSConfigFromContext(ctx)
		queueName := awsconfig.GetEnvOrDefault("AWS_SQS_QUEUE_NAME", "eventing-e2e-sqs-sink")

		secretName := feature.MakeRandomK8sName("aws-credentials")
		awsconfig.InstallAWSSecret(ctx, t, secretName)
		integrationsink.Install(sinkName, integrationsink.WithSQSSink(queueName, cfg.Region, secretName))(ctx, t)

	case integrationsink.SinkTypeSNS:
		cfg := awsconfig.AWSConfigFromContext(ctx)
		topicName := awsconfig.GetEnvOrDefault("AWS_SNS_TOPIC_NAME", "eventing-e2e-sns-sink")

		secretName := feature.MakeRandomK8sName("aws-credentials")
		awsconfig.InstallAWSSecret(ctx, t, secretName)
		integrationsink.Install(sinkName, integrationsink.WithSNSSink(topicName, cfg.Region, secretName))(ctx, t)

	case integrationsink.SinkTypeLog:
		integrationsink.Install(sinkName, integrationsink.WithLogSink())(ctx, t)
	}
}

func Success(sinkType integrationsink.SinkType) *feature.Feature {
	f := feature.NewFeature()

	integrationSink := feature.MakeRandomK8sName("integrationsink")
	source := feature.MakeRandomK8sName("source")

	f.Setup("install integration sink", func(ctx context.Context, t feature.T) {
		installSinkByType(ctx, t, integrationSink, sinkType)
	})

	f.Setup("integrationsink is addressable", integrationsink.IsAddressable(integrationSink))
	f.Setup("integrationsink is ready", integrationsink.IsReady(integrationSink))

	f.Requirement("install source for ksink", eventshub.Install(source,
		eventshub.StartSenderToResource(integrationsink.GVR(), integrationSink),
		eventshub.InputEvent(cetest.FullEvent()),
		eventshub.AddSequence,
		eventshub.SendMultipleEvents(2, time.Millisecond)))

	f.Assert("Source sent the event", assert.OnStore(source).
		Match(assert.MatchKind(eventshub.EventResponse)).
		Match(assert.MatchStatusCode(204)).
		AtLeast(1),
	)

	// Sink-specific setup, assertions, and teardown
	switch sinkType {
	case integrationsink.SinkTypeS3:
		f.Setup("cleanup S3 bucket before test", func(ctx context.Context, t feature.T) {
			arn := awsconfig.GetEnvOrDefault("AWS_S3_SINK_ARN", "arn:aws:s3:::eventing-e2e-sink")
			awsconfig.CleanupS3Bucket(ctx, t, arn)
		})

		f.Assert("verify S3 object count", func(ctx context.Context, t feature.T) {
			arn := awsconfig.GetEnvOrDefault("AWS_S3_SINK_ARN", "arn:aws:s3:::eventing-e2e-sink")
			// Verify 2 S3 objects were created (SendMultipleEvents(2, ...))
			awsconfig.VerifyS3ObjectCount(ctx, t, arn, 2)
		})

		f.Teardown("cleanup S3 bucket after test", func(ctx context.Context, t feature.T) {
			arn := awsconfig.GetEnvOrDefault("AWS_S3_SINK_ARN", "arn:aws:s3:::eventing-e2e-sink")
			awsconfig.CleanupS3Bucket(ctx, t, arn)
		})

	case integrationsink.SinkTypeSQS:
		f.Setup("cleanup SQS queue before test", func(ctx context.Context, t feature.T) {
			queueName := awsconfig.GetEnvOrDefault("AWS_SQS_QUEUE_NAME", "eventing-e2e-sqs-sink")
			queueURL := awsconfig.GetSQSQueueURL(ctx, t, queueName)
			awsconfig.CleanupSQSQueue(ctx, t, queueURL)
		})

		f.Assert("verify SQS messages", func(ctx context.Context, t feature.T) {
			queueName := awsconfig.GetEnvOrDefault("AWS_SQS_QUEUE_NAME", "eventing-e2e-sqs-sink")
			queueURL := awsconfig.GetSQSQueueURL(ctx, t, queueName)
			// Verify 2 SQS messages were created with expected content (SendMultipleEvents(2, ...))
			// Messages are deleted immediately after verification
			awsconfig.VerifySQSMessages(ctx, t, queueURL, "hello", 2)
		})

	case integrationsink.SinkTypeSNS:
		f.Setup("cleanup SNS verification queue before test", func(ctx context.Context, t feature.T) {
			queueName := awsconfig.GetEnvOrDefault("AWS_SNS_VERIFICATION_QUEUE_NAME", "eventing-e2e-sns-verification")
			queueURL := awsconfig.GetSQSQueueURL(ctx, t, queueName)
			awsconfig.CleanupSQSQueue(ctx, t, queueURL)
		})

		f.Assert("verify SNS messages", func(ctx context.Context, t feature.T) {
			queueName := awsconfig.GetEnvOrDefault("AWS_SNS_VERIFICATION_QUEUE_NAME", "eventing-e2e-sns-verification")
			queueURL := awsconfig.GetSQSQueueURL(ctx, t, queueName)
			// Verify 2 SNS messages were created with expected content (SendMultipleEvents(2, ...))
			// SNS messages are delivered to the SQS verification queue
			// Messages are deleted immediately after verification
			awsconfig.VerifySNSMessages(ctx, t, queueURL, "hello", 2)
		})
	}

	return f
}

func SuccessTLS(sinkType integrationsink.SinkType) *feature.Feature {
	f := feature.NewFeature()

	//	sink := feature.MakeRandomK8sName("sink")
	integrationSink := feature.MakeRandomK8sName("integrationsink")
	source := feature.MakeRandomK8sName("source")

	//sinkURL := &apis.URL{Scheme: "http", Host: sink}

	event := cetest.FullEvent()
	event.SetID(uuid.NewString())

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install integration sink", func(ctx context.Context, t feature.T) {
		installSinkByType(ctx, t, integrationSink, sinkType)
	})

	f.Setup("integrationsink is addressable", integrationsink.IsAddressable(integrationSink))
	f.Setup("integrationsink is ready", integrationsink.IsReady(integrationSink))

	f.Requirement("install source for ksink", eventshub.Install(source,
		eventshub.StartSenderToResource(integrationsink.GVR(), integrationSink),
		eventshub.InputEvent(cetest.FullEvent()),
		eventshub.AddSequence,
		eventshub.SendMultipleEvents(2, time.Millisecond)))

	f.Assert("IntegrationSink has https address", addressable.ValidateAddress(integrationsink.GVR(), integrationSink, addressable.AssertHTTPSAddress))

	f.Assert("Source sent the event", assert.OnStore(source).
		Match(assert.MatchKind(eventshub.EventResponse)).
		Match(assert.MatchStatusCode(204)).
		AtLeast(1),
	)

	return f
}
