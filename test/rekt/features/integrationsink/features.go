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
	"github.com/aws/aws-sdk-go-v2/service/sqs"
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
	case integrationsink.SinkTypeSQS:
		accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
		secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

		if accessKey == "" || secretKey == "" {
			t.Fatal("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables must be set for SQS sink tests")
		}

		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-west-1"
		}

		queueName := os.Getenv("AWS_SQS_QUEUE_NAME")
		if queueName == "" {
			queueName = "eventing-e2e-sqs-sink"
		}

		secretName := feature.MakeRandomK8sName("aws-credentials")
		secret.Install(
			secretName,
			secret.WithStringData("aws.accessKey", accessKey),
			secret.WithStringData("aws.secretKey", secretKey),
		)(ctx, t)
		integrationsink.Install(sinkName, integrationsink.WithSQSSink(queueName, region, secretName))(ctx, t)
	case integrationsink.SinkTypeSNS:
		accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
		secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

		if accessKey == "" || secretKey == "" {
			t.Fatal("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables must be set for SNS sink tests")
		}

		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-west-1"
		}

		topicName := os.Getenv("AWS_SNS_TOPIC_NAME")
		if topicName == "" {
			topicName = "eventing-e2e-sns-sink"
		}

		secretName := feature.MakeRandomK8sName("aws-credentials")
		secret.Install(
			secretName,
			secret.WithStringData("aws.accessKey", accessKey),
			secret.WithStringData("aws.secretKey", secretKey),
		)(ctx, t)
		integrationsink.Install(sinkName, integrationsink.WithSNSSink(topicName, region, secretName))(ctx, t)
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

func createSQSClient(ctx context.Context, t feature.T) *sqs.Client {
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

	return sqs.NewFromConfig(cfg)
}

// resolves a queue name to its full URL using the AWS SQS API
func getSQSQueueURL(ctx context.Context, t feature.T, queueName string) string {
	client := createSQSClient(ctx, t)

	result, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatalf("Failed to get SQS queue URL for queue '%s': %v", queueName, err)
	}

	if result.QueueUrl == nil {
		t.Fatalf("SQS queue URL is nil for queue '%s'", queueName)
	}

	return *result.QueueUrl
}

// removes all messages from SQS queue
func cleanupSQSQueue(ctx context.Context, t feature.T, queueURL string) {
	client := createSQSClient(ctx, t)

	// Receive and delete messages in batches
	for {
		receiveResult, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 10, // Max allowed
			WaitTimeSeconds:     1,  // Short poll
		})
		if err != nil {
			t.Logf("Warning: Failed to receive SQS messages: %v", err)
			return
		}

		if len(receiveResult.Messages) == 0 {
			t.Logf("No SQS messages found, queue is clean")
			break
		}

		t.Logf("Cleaning up %d SQS messages", len(receiveResult.Messages))
		for _, msg := range receiveResult.Messages {
			_, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: msg.ReceiptHandle,
			})
			if err != nil {
				t.Logf("Warning: Failed to delete SQS message: %v", err)
			} else {
				t.Logf("Cleaned up SQS message: %s", *msg.MessageId)
			}
		}
	}
}

// polls until the expected number of messages exist in SQS queue and verifies message bodies
// deletes messages immediately after verification
func verifySQSMessages(ctx context.Context, t feature.T, queueURL string, expectedContent string, expectedCount int) {
	client := createSQSClient(ctx, t)

	// Poll for expected message count
	interval := 2 * time.Second
	timeout := 2 * time.Minute

	var verifiedCount int
	err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		receiveResult, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     5,
		})
		if err != nil {
			// Continue polling on error
			return false, nil
		}

		// Process and delete messages immediately
		for _, msg := range receiveResult.Messages {
			if msg.Body != nil && strings.Contains(*msg.Body, expectedContent) {
				verifiedCount++
				_, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(queueURL),
					ReceiptHandle: msg.ReceiptHandle,
				})
				if err != nil {
					t.Logf("Warning: Failed to delete SQS message: %v", err)
				}
			}
		}

		return verifiedCount >= expectedCount, nil
	})

	if err != nil {
		t.Fatalf("Timeout waiting for %d SQS messages. Found: %d", expectedCount, verifiedCount)
	}

	if verifiedCount < expectedCount {
		t.Fatalf("Expected %d SQS messages, found: %d", expectedCount, verifiedCount)
	}

	t.Logf("Successfully verified %d SQS messages containing expected data", verifiedCount)
}

// polls until the expected number of SNS messages exist in the verification SQS queue
// SNS messages are delivered to SQS wrapped in an SNS envelope, so we check the Message field
// deletes messages immediately after verification
func verifySNSMessages(ctx context.Context, t feature.T, queueURL string, expectedContent string, expectedCount int) {
	client := createSQSClient(ctx, t)

	// Poll for expected message count
	interval := 2 * time.Second
	timeout := 2 * time.Minute

	var verifiedCount int
	err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		receiveResult, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     5,
		})
		if err != nil {
			// Continue polling on error
			return false, nil
		}

		// Process and delete messages immediately
		// SNS messages are wrapped in an envelope. The actual message is in the "Message" field.
		// We check if the message body (which is the SNS envelope) contains the expected content
		for _, msg := range receiveResult.Messages {
			if msg.Body != nil && strings.Contains(*msg.Body, expectedContent) {
				verifiedCount++
				_, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(queueURL),
					ReceiptHandle: msg.ReceiptHandle,
				})
				if err != nil {
					t.Logf("Warning: Failed to delete SNS message from verification queue: %v", err)
				}
			}
		}

		return verifiedCount >= expectedCount, nil
	})

	if err != nil {
		t.Fatalf("Timeout waiting for %d SNS messages in verification queue. Found: %d", expectedCount, verifiedCount)
	}

	if verifiedCount < expectedCount {
		t.Fatalf("Expected %d SNS messages in verification queue, found: %d", expectedCount, verifiedCount)
	}

	t.Logf("Successfully verified %d SNS messages containing expected data", verifiedCount)
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
			arn := os.Getenv("AWS_S3_SINK_ARN")
			if arn == "" {
				arn = "arn:aws:s3:::eventing-e2e-sink"
			}
			cleanupS3Bucket(ctx, t, arn)
		})

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

	case integrationsink.SinkTypeSQS:
		f.Setup("cleanup SQS queue before test", func(ctx context.Context, t feature.T) {
			queueName := os.Getenv("AWS_SQS_QUEUE_NAME")
			if queueName == "" {
				queueName = "eventing-e2e-sqs-sink"
			}
			queueURL := getSQSQueueURL(ctx, t, queueName)
			cleanupSQSQueue(ctx, t, queueURL)
		})

		f.Assert("verify SQS messages", func(ctx context.Context, t feature.T) {
			queueName := os.Getenv("AWS_SQS_QUEUE_NAME")
			if queueName == "" {
				queueName = "eventing-e2e-sqs-sink"
			}
			queueURL := getSQSQueueURL(ctx, t, queueName)
			// Verify 2 SQS messages were created with expected content (SendMultipleEvents(2, ...))
			// Messages are deleted immediately after verification
			verifySQSMessages(ctx, t, queueURL, "hello", 2)
		})

	case integrationsink.SinkTypeSNS:
		f.Setup("cleanup SNS verification queue before test", func(ctx context.Context, t feature.T) {
			queueName := os.Getenv("AWS_SNS_VERIFICATION_QUEUE_NAME")
			if queueName == "" {
				queueName = "eventing-e2e-sns-verification"
			}
			queueURL := getSQSQueueURL(ctx, t, queueName)
			cleanupSQSQueue(ctx, t, queueURL)
		})

		f.Assert("verify SNS messages", func(ctx context.Context, t feature.T) {
			queueName := os.Getenv("AWS_SNS_VERIFICATION_QUEUE_NAME")
			if queueName == "" {
				queueName = "eventing-e2e-sns-verification"
			}
			queueURL := getSQSQueueURL(ctx, t, queueName)
			// Verify 2 SNS messages were created with expected content (SendMultipleEvents(2, ...))
			// SNS messages are delivered to the SQS verification queue
			// Messages are deleted immediately after verification
			verifySNSMessages(ctx, t, queueURL, "hello", 2)
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
