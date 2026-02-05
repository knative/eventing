# Reconciler Tests

This is the staging location for the new
[e2e testing framework](https://github.com/knative-extensions/reconciler-test).

To run the tests on an existing cluster:

```bash
SYSTEM_NAMESPACE=knative-eventing go test -count=1 -v -tags=e2e ./test/rekt/...
```

To run just one test:

```bash
SYSTEM_NAMESPACE=knative-eventing go test -count=1 -v -tags=e2e -run Smoke_PingSource ./test/rekt/...
```

## AWS Integration tests

AWS integration tests validate IntegrationSource and IntegrationSink resources
that interact with AWS services (S3, SQS, SNS, DynamoDB Streams). These tests
use the `e2e_aws` build tag to separate them from regular e2e tests.

To run only AWS integration tests:

```bash
SYSTEM_NAMESPACE=knative-eventing \
  AWS_ACCESS_KEY_ID=<your-access-key> \
  AWS_SECRET_ACCESS_KEY=<your-secret-key> \
  go test -count=1 -v -tags=e2e_aws ./test/rekt/...
```

To run a specific AWS integration test:

```bash
SYSTEM_NAMESPACE=knative-eventing \
  AWS_ACCESS_KEY_ID=<your-access-key> \
  AWS_SECRET_ACCESS_KEY=<your-secret-key> \
  go test -count=1 -v -tags=e2e_aws -run TestIntegrationSinkS3Success ./test/rekt/...
```

### Required environment variables

- `AWS_ACCESS_KEY_ID` - AWS access key with permissions for S3, SQS, SNS, and DynamoDB
- `AWS_SECRET_ACCESS_KEY` - AWS secret key

### Optional environment variables

- `AWS_REGION` - AWS region (default: `us-west-1`)

#### IntegrationSource specific

- `AWS_S3_SOURCE_ARN` - S3 bucket ARN for source tests (default: `arn:aws:s3:::eventing-e2e-source`)
- `AWS_SQS_SOURCE_ARN` - SQS queue ARN for source tests (default: `arn:aws:sqs:us-west-1::eventing-e2e-sqs-source`)
- `AWS_DDB_STREAMS_TABLE` - DynamoDB table name for stream tests (default: `eventing-e2e-source`)

#### IntegrationSink specific

- `AWS_S3_SINK_ARN` - S3 bucket ARN for sink tests (default: `arn:aws:s3:::eventing-e2e-sink`)
- `AWS_SQS_QUEUE_NAME` - SQS queue name for sink tests (default: `eventing-e2e-sqs-sink`)
- `AWS_SNS_TOPIC_NAME` - SNS topic name for sink tests (default: `eventing-e2e-sns-sink`)
- `AWS_SNS_VERIFICATION_QUEUE_NAME` - SQS queue name for SNS message verification (default: `eventing-e2e-sns-verification`)

**Note:** The AWS resources (S3 buckets, SQS queues, SNS topics, DynamoDB tables)
must be created before running the tests. The tests will clean up objects/messages
created during test execution, but will not create or delete the AWS resources themselves.

### Setting up AWS resources

You can use the following script to create all required AWS resources:

```bash
#!/bin/bash
# setup-aws-resources.sh - Create AWS resources for integration tests

set -e

# Configuration
AWS_REGION="${AWS_REGION:-us-west-1}"
S3_SOURCE_BUCKET="eventing-e2e-source"
S3_SINK_BUCKET="eventing-e2e-sink"
SQS_SOURCE_QUEUE="eventing-e2e-sqs-source"
SQS_SINK_QUEUE="eventing-e2e-sqs-sink"
SNS_VERIFICATION_QUEUE="eventing-e2e-sns-verification"
SNS_TOPIC="eventing-e2e-sns-sink"
DDB_TABLE="eventing-e2e-source"

echo "Creating AWS resources in region: $AWS_REGION"

# Create S3 buckets
echo "Creating S3 buckets..."
aws s3api create-bucket \
  --bucket "$S3_SOURCE_BUCKET" \
  --region "$AWS_REGION" \
  --create-bucket-configuration LocationConstraint="$AWS_REGION" 2>/dev/null || echo "Bucket $S3_SOURCE_BUCKET already exists"

aws s3api create-bucket \
  --bucket "$S3_SINK_BUCKET" \
  --region "$AWS_REGION" \
  --create-bucket-configuration LocationConstraint="$AWS_REGION" 2>/dev/null || echo "Bucket $S3_SINK_BUCKET already exists"

# Create SQS queues
echo "Creating SQS queues..."
aws sqs create-queue \
  --queue-name "$SQS_SOURCE_QUEUE" \
  --region "$AWS_REGION" >/dev/null || echo "Queue $SQS_SOURCE_QUEUE already exists"

aws sqs create-queue \
  --queue-name "$SQS_SINK_QUEUE" \
  --region "$AWS_REGION" >/dev/null || echo "Queue $SQS_SINK_QUEUE already exists"

aws sqs create-queue \
  --queue-name "$SNS_VERIFICATION_QUEUE" \
  --region "$AWS_REGION" >/dev/null || echo "Queue $SNS_VERIFICATION_QUEUE already exists"

# Get queue ARN for SNS subscription
VERIFICATION_QUEUE_URL=$(aws sqs get-queue-url \
  --queue-name "$SNS_VERIFICATION_QUEUE" \
  --region "$AWS_REGION" \
  --query 'QueueUrl' \
  --output text)

VERIFICATION_QUEUE_ARN=$(aws sqs get-queue-attributes \
  --queue-url "$VERIFICATION_QUEUE_URL" \
  --attribute-names QueueArn \
  --region "$AWS_REGION" \
  --query 'Attributes.QueueArn' \
  --output text)

# Create SNS topic
echo "Creating SNS topic..."
SNS_TOPIC_ARN=$(aws sns create-topic \
  --name "$SNS_TOPIC" \
  --region "$AWS_REGION" \
  --query 'TopicArn' \
  --output text)

echo "SNS Topic ARN: $SNS_TOPIC_ARN"

# Set SQS queue policy to allow SNS to send messages
echo "Setting SQS queue policy for SNS..."
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

QUEUE_POLICY=$(cat <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "sns.amazonaws.com"
      },
      "Action": "SQS:SendMessage",
      "Resource": "$VERIFICATION_QUEUE_ARN",
      "Condition": {
        "ArnEquals": {
          "aws:SourceArn": "$SNS_TOPIC_ARN"
        }
      }
    }
  ]
}
EOF
)

POLICY_STRING=$(echo "$QUEUE_POLICY" | jq -c . | jq -R .)
aws sqs set-queue-attributes \
  --queue-url "$VERIFICATION_QUEUE_URL" \
  --attributes "{\"Policy\":$POLICY_STRING}" \
  --region "$AWS_REGION"

# Subscribe SQS queue to SNS topic
echo "Subscribing SQS queue to SNS topic..."
SUBSCRIPTION_ARN=$(aws sns subscribe \
  --topic-arn "$SNS_TOPIC_ARN" \
  --protocol sqs \
  --notification-endpoint "$VERIFICATION_QUEUE_ARN" \
  --region "$AWS_REGION" \
  --query 'SubscriptionArn' \
  --output text)

echo "Subscription ARN: $SUBSCRIPTION_ARN"

# Create DynamoDB table with streams enabled
echo "Creating DynamoDB table with streams..."
aws dynamodb create-table \
  --table-name "$DDB_TABLE" \
  --attribute-definitions AttributeName=id,AttributeType=S \
  --key-schema AttributeName=id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --stream-specification StreamEnabled=true,StreamViewType=NEW_AND_OLD_IMAGES \
  --region "$AWS_REGION" >/dev/null 2>&1 || echo "Table $DDB_TABLE already exists"

# Wait for DynamoDB table to be active
echo "Waiting for DynamoDB table to be active..."
aws dynamodb wait table-exists \
  --table-name "$DDB_TABLE" \
  --region "$AWS_REGION"

echo "All AWS resources created successfully!"
echo ""
echo "Environment variables for tests:"
echo "export AWS_REGION=$AWS_REGION"
echo "export AWS_S3_SOURCE_ARN=arn:aws:s3:::$S3_SOURCE_BUCKET"
echo "export AWS_S3_SINK_ARN=arn:aws:s3:::$S3_SINK_BUCKET"
echo "export AWS_SQS_SOURCE_ARN=arn:aws:sqs:$AWS_REGION:$ACCOUNT_ID:$SQS_SOURCE_QUEUE"
echo "export AWS_SQS_QUEUE_NAME=$SQS_SINK_QUEUE"
echo "export AWS_SNS_TOPIC_NAME=$SNS_TOPIC"
echo "export AWS_SNS_VERIFICATION_QUEUE_NAME=$SNS_VERIFICATION_QUEUE"
echo "export AWS_DDB_STREAMS_TABLE=$DDB_TABLE"
```

### Tearing down AWS resources

You can use the following script to delete all AWS resources created for testing:

```bash
#!/bin/bash
# teardown-aws-resources.sh - Delete AWS resources for integration tests

set -e

# Configuration
AWS_REGION="${AWS_REGION:-us-west-1}"
S3_SOURCE_BUCKET="eventing-e2e-source"
S3_SINK_BUCKET="eventing-e2e-sink"
SQS_SOURCE_QUEUE="eventing-e2e-sqs-source"
SQS_SINK_QUEUE="eventing-e2e-sqs-sink"
SNS_VERIFICATION_QUEUE="eventing-e2e-sns-verification"
SNS_TOPIC="eventing-e2e-sns-sink"
DDB_TABLE="eventing-e2e-source"

echo "Deleting AWS resources in region: $AWS_REGION"

# Delete S3 buckets (must empty first)
echo "Deleting S3 buckets..."
aws s3 rm "s3://$S3_SOURCE_BUCKET" --recursive --region "$AWS_REGION" 2>/dev/null || true
aws s3api delete-bucket \
  --bucket "$S3_SOURCE_BUCKET" \
  --region "$AWS_REGION" 2>/dev/null || echo "Bucket $S3_SOURCE_BUCKET not found"

aws s3 rm "s3://$S3_SINK_BUCKET" --recursive --region "$AWS_REGION" 2>/dev/null || true
aws s3api delete-bucket \
  --bucket "$S3_SINK_BUCKET" \
  --region "$AWS_REGION" 2>/dev/null || echo "Bucket $S3_SINK_BUCKET not found"

# Get SNS topic ARN and unsubscribe SQS queue
echo "Unsubscribing SQS from SNS..."
SNS_TOPIC_ARN=$(aws sns list-topics \
  --region "$AWS_REGION" \
  --query "Topics[?contains(TopicArn, '$SNS_TOPIC')].TopicArn" \
  --output text 2>/dev/null || true)

if [ -n "$SNS_TOPIC_ARN" ]; then
  SUBSCRIPTIONS=$(aws sns list-subscriptions-by-topic \
    --topic-arn "$SNS_TOPIC_ARN" \
    --region "$AWS_REGION" \
    --query 'Subscriptions[].SubscriptionArn' \
    --output text 2>/dev/null || true)

  for SUB_ARN in $SUBSCRIPTIONS; do
    if [ "$SUB_ARN" != "PendingConfirmation" ]; then
      aws sns unsubscribe \
        --subscription-arn "$SUB_ARN" \
        --region "$AWS_REGION" 2>/dev/null || true
    fi
  done
fi

# Delete SNS topic
echo "Deleting SNS topic..."
if [ -n "$SNS_TOPIC_ARN" ]; then
  aws sns delete-topic \
    --topic-arn "$SNS_TOPIC_ARN" \
    --region "$AWS_REGION" 2>/dev/null || echo "SNS topic $SNS_TOPIC not found"
fi

# Delete SQS queues
echo "Deleting SQS queues..."
for QUEUE in "$SQS_SOURCE_QUEUE" "$SQS_SINK_QUEUE" "$SNS_VERIFICATION_QUEUE"; do
  QUEUE_URL=$(aws sqs get-queue-url \
    --queue-name "$QUEUE" \
    --region "$AWS_REGION" \
    --query 'QueueUrl' \
    --output text 2>/dev/null || true)

  if [ -n "$QUEUE_URL" ]; then
    aws sqs delete-queue \
      --queue-url "$QUEUE_URL" \
      --region "$AWS_REGION" 2>/dev/null || echo "Queue $QUEUE not found"
  fi
done

# Delete DynamoDB table
echo "Deleting DynamoDB table..."
aws dynamodb delete-table \
  --table-name "$DDB_TABLE" \
  --region "$AWS_REGION" >/dev/null 2>&1 || echo "Table $DDB_TABLE not found"

# Wait for DynamoDB table to be deleted
echo "Waiting for DynamoDB table deletion..."
aws dynamodb wait table-not-exists \
  --table-name "$DDB_TABLE" \
  --region "$AWS_REGION" 2>/dev/null || true

echo "All AWS resources deleted successfully!"
```

## Broker tests.

The Broker class can be overridden by using the envvar `BROKER_CLASS`. By
default, this will be `MTChannelBasedBroker`.

The Broker templates can be overridden by using the env var `BROKER_TEMPLATES`.

```bash
BROKER_CLASS=MyCustomBroker
BROKER_TEMPLATES=/path/to/custom/templates
SYSTEM_NAMESPACE=knative-eventing \
  go test -count=1 -v -tags=e2e -run Smoke_PingSource ./test/rekt/...
```

### Custom templates

The minimum shape of a custom template for namespaced resources:

```yaml
apiVersion: rando.api/v1
kind: MyResource
metadata:
  name: { { .name } }
  namespace: { { .namespace } }
spec:
  any:
    thing: that
  is: required
```

See [./resources](./resources) for examples.
