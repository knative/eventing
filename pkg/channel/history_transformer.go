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

package channel

import (
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	"github.com/cloudevents/sdk-go/v2/types"
)

func AddHistory(host string) binding.Transformer {
	return transformer.SetExtension(EventHistory, func(i interface{}) (interface{}, error) {
		if types.IsZero(i) {
			return host, nil
		}
		str, err := types.Format(i)
		if err != nil {
			return nil, err
		}
		h := decodeEventHistory(str)
		h = append(h, host)
		return encodeEventHistory(h), nil
	})
}
