/*
Copyright 2020 The Knative Authors

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

package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"
)

func (sink *IntegrationSink) Validate(ctx context.Context) *apis.FieldError {
	ctx = apis.WithinParent(ctx, sink.ObjectMeta)
	return sink.Spec.Validate(ctx).ViaField("spec")
}

func (spec *IntegrationSinkSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// Count how many fields are set to ensure mutual exclusivity
	sinkSetCount := 0
	if spec.Log != nil {
		sinkSetCount++
	}
	if spec.Aws != nil {
		if spec.Aws.S3 != nil {
			sinkSetCount++
		}
		if spec.Aws.SQS != nil {
			sinkSetCount++
		}
		if spec.Aws.SNS != nil {
			sinkSetCount++
		}
	}

	// Validate that only one sink field is set
	if sinkSetCount > 1 {
		errs = errs.Also(apis.ErrGeneric("only one sink type can be set", "spec"))
	} else if sinkSetCount == 0 {
		errs = errs.Also(apis.ErrGeneric("at least one sink type must be specified", "spec"))
	}

	// Only perform AWS-specific validation if exactly one AWS sink is configured
	if sinkSetCount == 1 && spec.Aws != nil {
		if spec.Aws.S3 != nil || spec.Aws.SQS != nil || spec.Aws.SNS != nil {
			// Check that AWS Auth is properly configured
			if !spec.Aws.Auth.HasAuth() {
				errs = errs.Also(apis.ErrMissingField("aws.auth.secret.ref.name"))
			}
		}

		// Additional validation for AWS S3 required fields
		if spec.Aws.S3 != nil {
			if spec.Aws.S3.Arn == "" {
				errs = errs.Also(apis.ErrMissingField("aws.s3.arn"))
			}
			if spec.Aws.S3.Region == "" {
				errs = errs.Also(apis.ErrMissingField("aws.s3.region"))
			}
		}

		// Additional validation for AWS SQS required fields
		if spec.Aws.SQS != nil {
			if spec.Aws.SQS.Arn == "" {
				errs = errs.Also(apis.ErrMissingField("aws.sqs.arn"))
			}
			if spec.Aws.SQS.Region == "" {
				errs = errs.Also(apis.ErrMissingField("aws.sqs.region"))
			}
		}
		// Additional validation for AWS SNS required fields
		if spec.Aws.SNS != nil {
			if spec.Aws.SNS.Arn == "" {
				errs = errs.Also(apis.ErrMissingField("aws.sns.arn"))
			}
			if spec.Aws.SNS.Region == "" {
				errs = errs.Also(apis.ErrMissingField("aws.sns.region"))
			}
		}
	}

	return errs
}
