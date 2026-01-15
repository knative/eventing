/*
Copyright 2026 The Knative Authors

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

// Package awsconfig provides AWS configuration management
//
// Usage:
//
//	ctx, env := global.Environment(
//	    knative.WithKnativeNamespace(system.Namespace()),
//	    awsconfig.WithAWSConfig,  // ← Load AWS config into context
//	    environment.Managed(t),
//	)
//
// Then access AWS config in feature functions:
//
//	cfg := awsconfig.AWSConfigFromContext(ctx)
//	client := awsconfig.CreateS3Client(ctx)
package awsconfig

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"

	"knative.dev/eventing/test/rekt/resources/secret"
)

type AWSConfig struct {
	AccessKey string
	SecretKey string
	Region    string
	SDKConfig aws.Config // Cached AWS SDK config
}

// Context key
type awsConfigKey struct{}

// WithAWSConfig loads AWS configuration from environment variables and stores it in context.
//
// Environment variables:
//   - AWS_ACCESS_KEY_ID (required)
//   - AWS_SECRET_ACCESS_KEY (required)
//   - AWS_REGION (optional, default: us-west-1)
//
// Example:
//
//	ctx, env := global.Environment(
//	    knative.WithKnativeNamespace(system.Namespace()),
//	    awsconfig.WithAWSConfig,
//	    environment.Managed(t),
//	)
func WithAWSConfig(ctx context.Context, env environment.Environment) (context.Context, error) {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

	if accessKey == "" || secretKey == "" {
		return ctx, errors.New("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables must be set")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-west-1"
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return ctx, fmt.Errorf("failed to load AWS config: %w", err)
	}

	awsConfig := &AWSConfig{
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    region,
		SDKConfig: cfg,
	}

	return context.WithValue(ctx, awsConfigKey{}, awsConfig), nil
}

// AWSConfigFromContext retrieves AWS configuration from context.
//
// Panics if AWS config is not found in context.
// Make sure to use WithAWSConfig when setting up the test environment.
func AWSConfigFromContext(ctx context.Context) *AWSConfig {
	if cfg, ok := ctx.Value(awsConfigKey{}).(*AWSConfig); ok {
		return cfg
	}
	panic("no AWS config found in context")
}

// GetEnvOrDefault reads an environment variable or returns the default value if not set.
// This is a utility function for reading service-specific environment variables.
//
// Example:
//
//	arn := awsconfig.GetEnvOrDefault("AWS_S3_SINK_ARN", "arn:aws:s3:::eventing-e2e-sink")
func GetEnvOrDefault(envVar, defaultValue string) string {
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return defaultValue
}

// CreateS3Client creates an S3 client using AWS configuration from context.
func CreateS3Client(ctx context.Context) *s3.Client {
	cfg := AWSConfigFromContext(ctx)
	return s3.NewFromConfig(cfg.SDKConfig)
}

// CreateSQSClient creates an SQS client using AWS configuration from context.
func CreateSQSClient(ctx context.Context) *sqs.Client {
	cfg := AWSConfigFromContext(ctx)
	return sqs.NewFromConfig(cfg.SDKConfig)
}

// CreateDynamoDBClient creates a DynamoDB client using AWS configuration from context.
func CreateDynamoDBClient(ctx context.Context) *dynamodb.Client {
	cfg := AWSConfigFromContext(ctx)
	return dynamodb.NewFromConfig(cfg.SDKConfig)
}

// InstallAWSSecret creates a Kubernetes secret with AWS credentials from context.
func InstallAWSSecret(ctx context.Context, t feature.T, secretName string) {
	cfg := AWSConfigFromContext(ctx)
	secret.Install(
		secretName,
		secret.WithStringData("aws.accessKey", cfg.AccessKey),
		secret.WithStringData("aws.secretKey", cfg.SecretKey),
	)(ctx, t)
}

// CleanupS3Bucket removes all objects from the specified S3 bucket.
// The arn parameter can be either a full ARN (arn:aws:s3:::bucket-name) or just the bucket name.
func CleanupS3Bucket(ctx context.Context, t feature.T, arn string) {
	// Extract bucket name from ARN
	bucketName := arn
	parts := strings.Split(arn, ":::")
	if len(parts) == 2 {
		bucketName = parts[1]
	}

	client := CreateS3Client(ctx)

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

// GetSQSQueueURL resolves a queue name to its full URL using the AWS SQS API.
func GetSQSQueueURL(ctx context.Context, t feature.T, queueName string) string {
	client := CreateSQSClient(ctx)

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

// CleanupSQSQueue removes all messages from the specified SQS queue.
func CleanupSQSQueue(ctx context.Context, t feature.T, queueURL string) {
	client := CreateSQSClient(ctx)

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

// CleanupDynamoDBTable removes all items from the specified DynamoDB table.
// Note: This implementation assumes "id" is the primary key of the table.
func CleanupDynamoDBTable(ctx context.Context, t feature.T, tableName string) {
	client := CreateDynamoDBClient(ctx)

	// Scan the table to get all items
	scanResult, err := client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		t.Logf("Warning: Failed to scan table %s: %v", tableName, err)
		return
	}

	if len(scanResult.Items) == 0 {
		t.Logf("No DynamoDB items found, table is clean")
		return
	}

	// Delete each item (assumes "id" is the primary key)
	t.Logf("Cleaning up %d DynamoDB items", len(scanResult.Items))
	for _, item := range scanResult.Items {
		if idAttr, ok := item["id"]; ok {
			_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
				TableName: aws.String(tableName),
				Key: map[string]types.AttributeValue{
					"id": idAttr,
				},
			})
			if err != nil {
				t.Logf("Warning: Failed to delete item from table: %v", err)
			} else {
				t.Logf("Cleaned up DynamoDB item with id: %v", idAttr)
			}
		}
	}
}

// UploadFileToS3 uploads a file to the specified S3 bucket.
// The arn parameter can be either a full ARN (arn:aws:s3:::bucket-name) or just the bucket name.
func UploadFileToS3(ctx context.Context, t feature.T, arn, key, content string) {
	// extract bucket name from S3 ARN
	// Example: arn:aws:s3:::my-bucket -> my-bucket
	bucketName := arn
	parts := strings.Split(arn, ":::")
	if len(parts) == 2 {
		bucketName = parts[1]
	}

	client := CreateS3Client(ctx)

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(content)),
	})
	if err != nil {
		t.Fatalf("Failed to upload file to S3: %v", err)
	}
}

// SendSQSMessage sends a message to the specified SQS queue.
// The arn parameter should be in the format: arn:aws:sqs:region:account:queue-name
func SendSQSMessage(ctx context.Context, t feature.T, arn, messageBody string) {
	client := CreateSQSClient(ctx)

	// extract queue name from SQS ARN
	// Example: arn:aws:sqs:us-west-1:123456789012:my-queue -> my-queue
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

// PutDynamoDBItem puts an item into the specified DynamoDB table.
// The item contains an "id" field (number) and a "message" field (string).
func PutDynamoDBItem(ctx context.Context, t feature.T, tableName, message string) {
	client := CreateDynamoDBClient(ctx)

	_, err := client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"id":      &types.AttributeValueMemberN{Value: "1"},
			"message": &types.AttributeValueMemberS{Value: message},
		},
	})
	if err != nil {
		t.Fatalf("Failed to put item to DynamoDB table %s: %v", tableName, err)
	}
}

// VerifyS3ObjectCount polls until the expected number of objects exist in the S3 bucket.
// The arn parameter can be either a full ARN (arn:aws:s3:::bucket-name) or just the bucket name.
func VerifyS3ObjectCount(ctx context.Context, t feature.T, arn string, expectedCount int) {
	// Extract bucket name from ARN
	bucketName := arn
	parts := strings.Split(arn, ":::")
	if len(parts) == 2 {
		bucketName = parts[1]
	}

	client := CreateS3Client(ctx)

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

// VerifySQSMessages polls until the expected number of messages exist in SQS queue and verifies message bodies.
// Messages are deleted immediately after verification.
func VerifySQSMessages(ctx context.Context, t feature.T, queueURL string, expectedContent string, expectedCount int) {
	client := CreateSQSClient(ctx)

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

// VerifySNSMessages polls until the expected number of SNS messages exist in the verification SQS queue.
// SNS messages are delivered to SQS wrapped in an SNS envelope, so we check the Message field.
// Messages are deleted immediately after verification.
func VerifySNSMessages(ctx context.Context, t feature.T, queueURL string, expectedContent string, expectedCount int) {
	client := CreateSQSClient(ctx)

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
