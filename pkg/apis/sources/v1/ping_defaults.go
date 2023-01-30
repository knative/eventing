/*
Copyright 2021 The Knative Authors

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

package v1

import (
	"context"
)

const (
	defaultSchedule   = "* * * * *"
	defaultDateLayout = "2006-01-02T15:04:05.000Z"
)

func (s *PingSource) SetDefaults(ctx context.Context) {
	s.Spec.SetDefaults(ctx)
}

func (ss *PingSourceSpec) SetDefaults(ctx context.Context) {
	if ss.Schedule == "" {
		ss.Schedule = defaultSchedule
	}
}
