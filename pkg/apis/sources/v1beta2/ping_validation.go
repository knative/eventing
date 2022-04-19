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

package v1beta2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/robfig/cron/v3"
	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/apis/sources/config"
)

func (c *PingSource) Validate(ctx context.Context) *apis.FieldError {
	return c.Spec.Validate(ctx).ViaField("spec")
}

func (cs *PingSourceSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	schedule := cs.Schedule

	errs = validateDescriptor(schedule)

	if cs.Timezone != "" {
		schedule = "CRON_TZ=" + cs.Timezone + " " + schedule
	}

	if _, err := cron.ParseStandard(schedule); err != nil {
		if strings.HasPrefix(err.Error(), "provided bad location") {
			fe := apis.ErrInvalidValue(err, "timezone")
			errs = errs.Also(fe)
		} else {
			fe := apis.ErrInvalidValue(err, "schedule")
			errs = errs.Also(fe)
		}
	}

	pingConfig := config.FromContextOrDefaults(ctx)
	pingDefaults := pingConfig.PingDefaults.GetPingConfig()

	if fe := cs.Sink.Validate(ctx); fe != nil {
		errs = errs.Also(fe.ViaField("sink"))
	}

	if cs.Data != "" && cs.DataBase64 != "" {
		errs = errs.Also(apis.ErrMultipleOneOf("data", "dataBase64"))
	} else if cs.DataBase64 != "" {
		if bsize := int64(len(cs.DataBase64)); pingDefaults.DataMaxSize > -1 && bsize > pingDefaults.DataMaxSize {
			fe := apis.ErrInvalidValue(fmt.Sprintf("the dataBase64 length of %d bytes exceeds limit set at %d.", bsize, pingDefaults.DataMaxSize), "dataBase64")
			errs = errs.Also(fe)
		}
		decoded, err := base64.StdEncoding.DecodeString(cs.DataBase64)
		// invalid base64 string
		if err != nil {
			errs = errs.Also(apis.ErrInvalidValue(err, "dataBase64"))
		} else {
			// validate if the decoded base64 string is valid JSON
			if cs.ContentType == cloudevents.ApplicationJSON {
				if err := validateJSON(string(decoded)); err != nil {
					errs = errs.Also(apis.ErrInvalidValue(err, "dataBase64"))
				}
			}
		}
	} else if cs.Data != "" {
		if bsize := int64(len(cs.Data)); pingDefaults.DataMaxSize > -1 && bsize > pingDefaults.DataMaxSize {
			fe := apis.ErrInvalidValue(fmt.Sprintf("the data length of %d bytes exceeds limit set at %d.", bsize, pingDefaults.DataMaxSize), "data")
			errs = errs.Also(fe)
		}
		if cs.ContentType == cloudevents.ApplicationJSON {
			// validate if data is valid JSON
			if err := validateJSON(cs.Data); err != nil {
				errs = errs.Also(apis.ErrInvalidValue(err, "data"))
			}
		}
	}
	errs = errs.Also(cs.SourceSpec.Validate(ctx))
	return errs
}

func validateJSON(str string) error {
	var objmap map[string]interface{}
	return json.Unmarshal([]byte(str), &objmap)
}

func validateDescriptor(spec string) *apis.FieldError {
	if strings.Contains(spec, "@every") {
		return apis.ErrInvalidValue(errors.New("unsupported descriptor @every"), "schedule")
	}
	return nil
}
