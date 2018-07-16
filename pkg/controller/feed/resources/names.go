/*
Copyright 2018 The Knative Authors

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

package resources

import (
	"fmt"

	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
)

// JobName returns the name of the job for the feed, taking into account whether
// it's been deleted.
func JobName(feed *feedsv1alpha1.Feed) string {
	if feed.GetDeletionTimestamp() != nil {
		return StopJobName(feed)
	}
	return StartJobName(feed)
}

// StartJobName returns the name of the job for the start operation.
func StartJobName(feed *feedsv1alpha1.Feed) string {
	return fmt.Sprintf("%s-start", feed.Name)
}

// StopJobName returns the name of the job for the stop operation.
func StopJobName(feed *feedsv1alpha1.Feed) string {
	return fmt.Sprintf("%s-stop", feed.Name)
}
