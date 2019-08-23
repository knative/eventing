/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package broker

import "go.opencensus.io/tag"

// MustNewTagKey creates a Tag or panics. This will only fail if the tag key
// doesn't conform to tag name validations.
// TODO OC library should provide this
func MustNewTagKey(k string) tag.Key {
	tagKey, err := tag.NewKey(k)
	if err != nil {
		panic(err)
	}
	return tagKey
}

// Buckets125 generates an array of buckets with approximate powers-of-two
// buckets that also aligns with powers of 10 on every 3rd step. This can
// be used to create a view.Distribution.
func Buckets125(low, high float64) []float64 {
	buckets := []float64{low}
	for last := low; last < high; last = last * 10 {
		buckets = append(buckets, 2*last, 5*last, 10*last)
	}
	return buckets
}
