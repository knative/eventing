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
	"time"

	"github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/test/rekt/features/awsconfig"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/features/source"
	"knative.dev/eventing/test/rekt/resources/integrationsource"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
)

// triggerEventByType triggers a message based on the source type
// For S3 sources, uploads a file to S3 bucket
// For SQS sources, sends a message to SQS queue
// For DynamoDB Streams sources, puts an item to DynamoDB table
// For Timer sources, does nothing (timer triggers automatically)
func triggerEventByType(ctx context.Context, t feature.T, sourceType integrationsource.SourceType) {
	switch sourceType {
	case integrationsource.SourceTypeS3:
		arn := awsconfig.GetEnvOrDefault("AWS_S3_SOURCE_ARN", "arn:aws:s3:::eventing-e2e-source")
		awsconfig.UploadFileToS3(ctx, t, arn, "message.json", `{"message": "Hello from AWS S3!"}`)
	case integrationsource.SourceTypeSQS:
		arn := awsconfig.GetEnvOrDefault("AWS_SQS_SOURCE_ARN", "arn:aws:sqs:us-west-1::eventing-e2e-sqs-source")
		awsconfig.SendSQSMessage(ctx, t, arn, `{"message": "Hello from AWS SQS!"}`)
	case integrationsource.SourceTypeDDbStreams:
		tableName := awsconfig.GetEnvOrDefault("AWS_DDB_STREAMS_TABLE", "eventing-e2e-source")
		awsconfig.PutDynamoDBItem(ctx, t, tableName, "Hello from AWS DynamoDB Streams!")
	case integrationsource.SourceTypeTimer:
		// Timer source triggers automatically, no action needed
	}
}

// installSourceByType installs an integration source based on the source type.
// For AWS sources (S3/SQS/DynamoDB Streams), it reads AWS credentials from environment variables and creates a secret.
// Environment variables:
//   - AWS_ACCESS_KEY_ID (required for S3/SQS/DynamoDB Streams sources)
//   - AWS_SECRET_ACCESS_KEY (required for S3/SQS/DynamoDB Streams sources)
//   - AWS_REGION (optional, default: us-west-1)
//   - AWS_S3_SOURCE_ARN (optional, default: arn:aws:s3:::eventing-e2e)
//   - AWS_SQS_SOURCE_ARN (optional, default: arn:aws:sqs:us-west-1::eventing-e2e-sqs-source)
//   - AWS_DDB_STREAMS_TABLE (optional, default: eventing-e2e-source)
func installSourceByType(ctx context.Context, t feature.T, sourceName string, sourceType integrationsource.SourceType, sinkOpts ...manifest.CfgFn) {
	switch sourceType {
	case integrationsource.SourceTypeS3:
		cfg := awsconfig.AWSConfigFromContext(ctx)
		arn := awsconfig.GetEnvOrDefault("AWS_S3_SOURCE_ARN", "arn:aws:s3:::eventing-e2e-source")

		secretName := feature.MakeRandomK8sName("aws-credentials")
		awsconfig.InstallAWSSecret(ctx, t, secretName)
		opts := append([]manifest.CfgFn{integrationsource.WithS3Source(arn, cfg.Region, secretName)}, sinkOpts...)
		integrationsource.Install(sourceName, opts...)(ctx, t)
	case integrationsource.SourceTypeSQS:
		cfg := awsconfig.AWSConfigFromContext(ctx)
		arn := awsconfig.GetEnvOrDefault("AWS_SQS_SOURCE_ARN", "arn:aws:sqs:us-west-1::eventing-e2e-sqs-source")

		secretName := feature.MakeRandomK8sName("aws-credentials")
		awsconfig.InstallAWSSecret(ctx, t, secretName)
		opts := append([]manifest.CfgFn{integrationsource.WithSQSSource(arn, cfg.Region, secretName)}, sinkOpts...)
		integrationsource.Install(sourceName, opts...)(ctx, t)
	case integrationsource.SourceTypeDDbStreams:
		cfg := awsconfig.AWSConfigFromContext(ctx)
		tableName := awsconfig.GetEnvOrDefault("AWS_DDB_STREAMS_TABLE", "eventing-e2e-source")

		secretName := feature.MakeRandomK8sName("aws-credentials")
		awsconfig.InstallAWSSecret(ctx, t, secretName)
		opts := append([]manifest.CfgFn{integrationsource.WithDynamoDBStreamsSource(tableName, cfg.Region, secretName)}, sinkOpts...)
		integrationsource.Install(sourceName, opts...)(ctx, t)
	case integrationsource.SourceTypeTimer:
		opts := append([]manifest.CfgFn{integrationsource.WithTimerSource()}, sinkOpts...)
		integrationsource.Install(sourceName, opts...)(ctx, t)
	}
}

func SendsEventsWithSinkRef(sourceType integrationsource.SourceType) *feature.Feature {
	sourceName := feature.MakeRandomK8sName("integrationsource")
	sinkName := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sinkName, eventshub.StartReceiver))

	f.Requirement("install integrationsource", func(ctx context.Context, t feature.T) {
		installSourceByType(ctx, t, sourceName, sourceType, integrationsource.WithSink(service.AsDestinationRef(sinkName)))
	})

	f.Requirement("integrationsource goes ready", integrationsource.IsReady(sourceName))

	// additional delay for DDBStreams as workaround for https://issues.redhat.com/browse/SRVKE-1834
	// even though pod with integrationsource is ready, it takes > 5 seconds until it starts receiving events
	if sourceType == integrationsource.SourceTypeDDbStreams {
		f.Requirement("sleep", func(ctx context.Context, t feature.T) {
			time.Sleep(10 * time.Second)
		})
	}

	f.Assert("trigger event", func(ctx context.Context, t feature.T) {
		triggerEventByType(ctx, t, sourceType)
	})

	f.Stable("integrationsource as event source").
		Must("delivers events",
			assert.OnStore(sinkName).MatchEvent(test.HasType(string(sourceType))).AtLeast(1))

	return f
}

func SendEventsWithTLSReceiverAsSink(sourceType integrationsource.SourceType) *feature.Feature {
	sourceName := feature.MakeRandomK8sName("integrationsource")
	sinkName := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sinkName, eventshub.StartReceiverTLS))

	f.Requirement("install ContainerSource", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sinkName)
		d.CACerts = eventshub.GetCaCerts(ctx)

		installSourceByType(ctx, t, sourceName, sourceType, integrationsource.WithSink(d))
	})
	f.Requirement("integrationsource goes ready", integrationsource.IsReady(sourceName))

	f.Assert("trigger event", func(ctx context.Context, t feature.T) {
		triggerEventByType(ctx, t, sourceType)
	})

	f.Stable("integrationsource as event source").
		Must("delivers events",
			assert.OnStore(sinkName).
				Match(assert.MatchKind(eventshub.EventReceived)).
				MatchEvent(test.HasType(string(sourceType))).
				AtLeast(1),
		).
		Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(integrationsource.Gvr(), sourceName)).
		Must("Set sinkCACerts to non empty CA certs", source.ExpectCACerts(integrationsource.Gvr(), sourceName))

	return f
}

func SendEventsWithTLSReceiverAsSinkTrustBundle(sourceType integrationsource.SourceType) *feature.Feature {
	sourceName := feature.MakeRandomK8sName("integrationsource")
	sinkName := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sinkName,
		eventshub.IssuerRef(eventingtlstesting.IssuerKind, eventingtlstesting.IssuerName),
		eventshub.StartReceiverTLS,
	))

	f.Requirement("install ContainerSource", func(ctx context.Context, t feature.T) {
		destinationSink := &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "https", // Force using https
				Host:   network.GetServiceHostname(sinkName, environment.FromContext(ctx).Namespace()),
			},
			CACerts: nil, // CA certs are in the trust-bundle
		}

		installSourceByType(ctx, t, sourceName, sourceType, integrationsource.WithSink(destinationSink))
	})
	f.Requirement("integrationsource goes ready", integrationsource.IsReady(sourceName))

	f.Assert("trigger event", func(ctx context.Context, t feature.T) {
		triggerEventByType(ctx, t, sourceType)
	})

	f.Stable("integrationsource as event source").
		Must("delivers events",
			assert.OnStore(sinkName).
				Match(assert.MatchKind(eventshub.EventReceived)).
				MatchEvent(test.HasType(string(sourceType))).
				AtLeast(1),
		).
		Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(integrationsource.Gvr(), sourceName))

	return f
}
