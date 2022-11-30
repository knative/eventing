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

package isoduration

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		iso         string
		validations []Validate
		want        time.Duration
		wantErr     bool
	}{
		{
			name:        "10ms",
			iso:         "PT0.01S",
			validations: []Validate{IsNonNegative, IsPositive},
			want:        10 * time.Millisecond,
			wantErr:     false,
		},
		{
			name:        "100ms",
			iso:         "PT0.1S",
			validations: []Validate{IsNonNegative, IsPositive},
			want:        100 * time.Millisecond,
			wantErr:     false,
		},
		{
			name:        "1s",
			iso:         "PT1S",
			validations: []Validate{IsNonNegative, IsPositive},
			want:        time.Second,
			wantErr:     false,
		},
		{
			name:        "9s",
			iso:         "PT9S",
			validations: []Validate{IsNonNegative, IsPositive},
			want:        9 * time.Second,
			wantErr:     false,
		},
		{
			name:        "99s",
			iso:         "PT99S",
			validations: []Validate{IsNonNegative, IsPositive},
			want:        99 * time.Second,
			wantErr:     false,
		},
		{
			name:        "1m",
			iso:         "PT1M",
			validations: []Validate{IsNonNegative, IsPositive},
			want:        time.Minute,
			wantErr:     false,
		},
		{
			name:        "1m in seconds",
			iso:         "PT60S",
			validations: []Validate{IsNonNegative, IsPositive},
			want:        time.Minute,
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.iso, tt.validations...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Parse() got = %v, want %v", got, tt.want)
			}
		})
	}
}
