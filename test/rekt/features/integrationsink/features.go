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
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/integrationsink"
	"knative.dev/eventing/test/rekt/resources/secret"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
)

// installs an integration sink based on the sink type.
// For AWS S3 sink, it reads AWS credentials from environment variables and creates a secret.
// Environment variables:
//   - AWS_ACCESS_KEY_ID (required for S3 sink)
//   - AWS_SECRET_ACCESS_KEY (required for S3 sink)
//   - AWS_REGION (optional, default: us-west-1)
//   - AWS_S3_SINK_ARN (optional, default: arn:aws:s3:::eventing-e2e-sink)
func installSinkByType(ctx context.Context, t feature.T, sinkName string, sinkType integrationsink.SinkType) {
	switch sinkType {
	case integrationsink.SinkTypeS3:
		accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
		secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

		if accessKey == "" || secretKey == "" {
			t.Fatal("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables must be set for S3 sink tests")
		}

		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-west-1"
		}

		arn := os.Getenv("AWS_S3_SINK_ARN")
		if arn == "" {
			arn = "arn:aws:s3:::eventing-e2e-sink"
		}

		secretName := feature.MakeRandomK8sName("aws-credentials")
		secret.Install(
			secretName,
			secret.WithStringData("aws.accessKey", accessKey),
			secret.WithStringData("aws.secretKey", secretKey),
		)(ctx, t)
		integrationsink.Install(sinkName, integrationsink.WithS3Sink(arn, region, secretName))(ctx, t)
	case integrationsink.SinkTypeLog:
		integrationsink.Install(sinkName, integrationsink.WithLogSink())(ctx, t)
	}
}

func createS3Client(ctx context.Context, t feature.T) *s3.Client {
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

	return s3.NewFromConfig(cfg)
}

// removes all objects from S3 bucket
func cleanupS3Bucket(ctx context.Context, t feature.T, arn string) {
	// Extract bucket name from ARN
	bucketName := arn
	parts := strings.Split(arn, ":::")
	if len(parts) == 2 {
		bucketName = parts[1]
	}

	client := createS3Client(ctx, t)

	// List all objects in bucket
	listResult, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Logf("Warning: Failed to list S3 objects: %v", err)
		return
	}

	if len(listResult.Contents) == 0 {
		t.Logf("No S3 objects found, bucket is clean")
		return
	}

	// Delete all objects
	t.Logf("Cleaning up %d S3 objects", len(listResult.Contents))
	for _, obj := range listResult.Contents {
		if obj.Key == nil {
			continue
		}
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    obj.Key,
		})
		if err != nil {
			t.Logf("Warning: Failed to cleanup S3 object %s: %v", *obj.Key, err)
		} else {
			t.Logf("Cleaned up S3 object: %s", *obj.Key)
		}
	}
}

// polls until the expected number of objects exist in the S3 bucket
func verifyS3ObjectCount(ctx context.Context, t feature.T, arn string, expectedCount int) {
	// Extract bucket name from ARN
	bucketName := arn
	parts := strings.Split(arn, ":::")
	if len(parts) == 2 {
		bucketName = parts[1]
	}

	client := createS3Client(ctx, t)

	// Poll for expected object count
	interval := 2 * time.Second
	timeout := 2 * time.Minute

	var objectCount int
	err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		listResult, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			// Continue polling on error
			return false, nil
		}

		objectCount = len(listResult.Contents)
		return objectCount == expectedCount, nil
	})

	if err != nil {
		t.Fatalf("Timeout waiting for %d S3 objects. Found: %d", expectedCount, objectCount)
	}

	if objectCount < expectedCount {
		t.Fatalf("Expected %d S3 objects, found: %d", expectedCount, objectCount)
	}

	t.Logf("Successfully verified %d S3 objects in bucket", objectCount)
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

	if sinkType == integrationsink.SinkTypeS3 {
		f.Setup("cleanup S3 bucket before test", func(ctx context.Context, t feature.T) {
			arn := os.Getenv("AWS_S3_SINK_ARN")
			if arn == "" {
				arn = "arn:aws:s3:::eventing-e2e-sink"
			}
			cleanupS3Bucket(ctx, t, arn)
		})
	}

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

	if sinkType == integrationsink.SinkTypeS3 {
		f.Assert("verify S3 object count", func(ctx context.Context, t feature.T) {
			arn := os.Getenv("AWS_S3_SINK_ARN")
			if arn == "" {
				arn = "arn:aws:s3:::eventing-e2e-sink"
			}
			// Verify 2 S3 objects were created (SendMultipleEvents(2, ...))
			verifyS3ObjectCount(ctx, t, arn, 2)
		})

		f.Teardown("cleanup S3 bucket after test", func(ctx context.Context, t feature.T) {
			arn := os.Getenv("AWS_S3_SINK_ARN")
			if arn == "" {
				arn = "arn:aws:s3:::eventing-e2e-sink"
			}
			cleanupS3Bucket(ctx, t, arn)
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
