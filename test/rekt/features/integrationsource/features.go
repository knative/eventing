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
	"bytes"
	"context"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/features/source"
	"knative.dev/eventing/test/rekt/resources/integrationsource"
	"knative.dev/eventing/test/rekt/resources/secret"
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

func uploadFileToS3(ctx context.Context, t feature.T, arn, key, content string) {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-west-1"
	}

	// extract bucket name from S3 ARN
	// Example: arn:aws:s3:::my-bucket -> my-bucket
	bucketName := arn
	parts := strings.Split(arn, ":::")
	if len(parts) == 2 {
		bucketName = parts[1]
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		t.Fatalf("Failed to load AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg)

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(content)),
	})
	if err != nil {
		t.Fatalf("Failed to upload file to S3: %v", err)
	}
}

func sendSQSEvent(ctx context.Context, t feature.T, arn, messageBody string) {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-west-1"
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		t.Fatalf("Failed to load AWS config: %v", err)
	}

	client := sqs.NewFromConfig(cfg)

	// extract queue name from S3 ARN
	// Example: arn:aws:sqs:us-west-1::my-queue -> my-queue
	parts := strings.Split(arn, ":")
	queueName := parts[5]

	// determine queue URL from queue name
	urlRes, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatalf("Failed to get queue url for %s, %v", queueName, err)
	}
	queueURL := urlRes.QueueUrl

	_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    queueURL,
		MessageBody: aws.String(messageBody),
	})
	if err != nil {
		t.Fatalf("Failed to send message to SQS: %v", err)
	}
}

// triggerEventByType triggers a message based on the source type
// For S3 sources, uploads a file to S3 bucket
// For SQS sources, sends a message to SQS queue
// For Timer sources, does nothing (timer triggers automatically)
func triggerEventByType(ctx context.Context, t feature.T, sourceType integrationsource.SourceType) {
	switch sourceType {
	case integrationsource.SourceTypeS3:
		arn := os.Getenv("AWS_S3_SOURCE_ARN")
		if arn == "" {
			arn = "arn:aws:s3:::eventing-e2e"
		}
		uploadFileToS3(ctx, t, arn, "message.json", `{"message": "Hello from AWS S3!"}`)
	case integrationsource.SourceTypeSQS:
		arn := os.Getenv("AWS_SQS_SOURCE_ARN")
		if arn == "" {
			arn = "arn:aws:sqs:us-west-1::eventing-e2e-sqs-source"
		}
		sendSQSEvent(ctx, t, arn, `{"message": "Hello from AWS SQS!"}`)
	case integrationsource.SourceTypeTimer:
		// Timer source triggers automatically, no action needed
	}
}

// installSourceByType installs an integration source based on the source type.
// For S3 sources, it reads AWS credentials from environment variables and creates a secret.
// Environment variables:
//   - AWS_ACCESS_KEY_ID (required for S3/SQS sources)
//   - AWS_SECRET_ACCESS_KEY (required for S3/SQS sources)
//   - AWS_REGION (optional, default: us-west-1)
//   - AWS_S3_SOURCE_ARN (optional, default: arn:aws:s3:::eventing-e2e)
//   - AWS_SQS_SOURCE_ARN (optional, default: arn:aws:sqs:us-west-1::eventing-e2e-sqs-source)
func installSourceByType(ctx context.Context, t feature.T, sourceName string, sourceType integrationsource.SourceType, sinkOpts ...manifest.CfgFn) {
	switch sourceType {
	case integrationsource.SourceTypeS3:
		accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
		secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

		if accessKey == "" || secretKey == "" {
			t.Fatal("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables must be set for S3 source tests")
		}

		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-west-1"
		}

		arn := os.Getenv("AWS_S3_SOURCE_ARN")
		if arn == "" {
			arn = "arn:aws:s3:::eventing-e2e"
		}

		secretName := feature.MakeRandomK8sName("aws-credentials")
		secret.Install(
			secretName,
			secret.WithStringData("aws.accessKey", accessKey),
			secret.WithStringData("aws.secretKey", secretKey),
		)(ctx, t)
		opts := append([]manifest.CfgFn{integrationsource.WithS3Source(arn, region, secretName)}, sinkOpts...)
		integrationsource.Install(sourceName, opts...)(ctx, t)
	case integrationsource.SourceTypeSQS:
		accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
		secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

		if accessKey == "" || secretKey == "" {
			t.Fatal("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables must be set for S3 source tests")
		}

		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-west-1"
		}

		arn := os.Getenv("AWS_SQS_SOURCE_ARN")
		if arn == "" {
			arn = "arn:aws:sqs:us-west-1::eventing-e2e-sqs-source"
		}

		secretName := feature.MakeRandomK8sName("aws-credentials")
		secret.Install(
			secretName,
			secret.WithStringData("aws.accessKey", accessKey),
			secret.WithStringData("aws.secretKey", secretKey),
		)(ctx, t)
		opts := append([]manifest.CfgFn{integrationsource.WithSQSSource(arn, region, secretName)}, sinkOpts...)
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

	f.Requirement("trigger event", func(ctx context.Context, t feature.T) {
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

	f.Requirement("trigger event", func(ctx context.Context, t feature.T) {
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

	f.Requirement("trigger event", func(ctx context.Context, t feature.T) {
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
