/*
Copyright 2023 The Knative Authors

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

package auth

import (
	"reflect"
	"testing"
	"time"
)

func TestGetJWTExpiry(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "Valid JWT",
			token:   "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJteS1pc3N1ZXIiLCJpYXQiOjE1Nzc5MzA2NDUsImV4cCI6MTU3NzkzNDI0NSwiYXVkIjoibXktYXVkaWVuY2UiLCJzdWIiOiJzdWJqZWN0QGV4YW1wbGUuY29tIn0.Hl8n6Ipt0X0gI46QLPZtpESRtc7cQ75AqXNal0sQ2a4",
			want:    time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC),
			wantErr: false,
		}, {
			name:    "No valid JWT",
			token:   "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJteS1pc3N1ZXIiLCJpYXQiOjE1Nzc5MzA2NDUsImV4cCI6MTU3NzkzNDI0NSwiYXVkIjoibXktYXVkaWVuY2UiLCJzdWIiOiJzdWJqZWN0QGV4YW1wbGUuY29.Hl8n6Ipt0X0gI46QLPZtpESRtc7cQ75AqXNal0sQ2a4",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetJWTExpiry(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetJWTExpiry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.UTC(), tt.want.UTC()) {
				t.Errorf("GetJWTExpiry() = %v, want %v", got.UTC(), tt.want.UTC())
			}
		})
	}
}
