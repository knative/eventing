/*
Copyright 2022 The Knative Authors

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

// Package isoduration provides common functions to work with ISO 8601 durations
package isoduration

import (
	"fmt"
	"time"

	"github.com/sosodev/duration"
)

type Validate func(time.Duration) error

func Parse(iso string, validations ...Validate) (time.Duration, error) {
	if iso == "" {
		return 0, fmt.Errorf("empty string as duration")
	}

	d, err := duration.Parse(iso)
	if err != nil {
		return 0, fmt.Errorf("failed to parse ISO 8601 duration %s: %w", iso, err)
	}

	td := d.ToTimeDuration()

	for _, v := range validations {
		if err := v(td); err != nil {
			return td, err
		}
	}

	return d.ToTimeDuration(), nil
}

func ToString(d time.Duration) string {
	dd := duration.Duration{Seconds: d.Seconds()}
	return dd.String()
}

func IsPositive(d time.Duration) error {
	if d.Nanoseconds() <= 0 {
		return fmt.Errorf("duration must be positive, got %s", d.String())
	}
	return nil
}

func IsNonNegative(d time.Duration) error {
	if d.Nanoseconds() < 0 {
		return fmt.Errorf("duration must not be negative, got %s", d.String())
	}
	return nil
}
