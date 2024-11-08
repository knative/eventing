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

func (source *IntegrationSource) Validate(ctx context.Context) *apis.FieldError {
	ctx = apis.WithinParent(ctx, source.ObjectMeta)
	return source.Spec.Validate(ctx).ViaField("spec")
}

func (spec *IntegrationSourceSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// Count how many fields are set to ensure mutual exclusivity
	sourceSetCount := 0
	if spec.Timer != nil {
		sourceSetCount++
	}
	if spec.Aws != nil {
		if spec.Aws.S3 != nil {
			sourceSetCount++
		}
		if spec.Aws.SQS != nil {
			sourceSetCount++
		}
		if spec.Aws.DDBStreams != nil {
			sourceSetCount++
		}
	}

	// Validate that only one source field is set
	if sourceSetCount > 1 {
		errs = errs.Also(apis.ErrGeneric("only one source type can be set", "spec"))
	} else if sourceSetCount == 0 {
		errs = errs.Also(apis.ErrGeneric("at least one source type must be specified", "spec"))
	}

	// Only perform AWS-specific validation if exactly one AWS source is configured
	if sourceSetCount == 1 && spec.Aws != nil {
		if spec.Aws.S3 != nil || spec.Aws.SQS != nil || spec.Aws.DDBStreams != nil {
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
				errs = errs.Also(apis.ErrMissingField("aws.sqs.queueNameOrArn"))
			}
			if spec.Aws.SQS.Region == "" {
				errs = errs.Also(apis.ErrMissingField("aws.sqs.region"))
			}
		}

		// Additional validation for AWS DDBStreams required fields
		if spec.Aws.DDBStreams != nil {
			if spec.Aws.DDBStreams.Table == "" {
				errs = errs.Also(apis.ErrMissingField("aws.ddb-streams.table"))
			}
			if spec.Aws.DDBStreams.Region == "" {
				errs = errs.Also(apis.ErrMissingField("aws.ddb-streams.region"))
			}
		}
	}

	return errs
}
