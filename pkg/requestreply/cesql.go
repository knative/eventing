/*
Copyright 2025 The Knative Authors

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

package requestreply

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	cesql "github.com/cloudevents/sdk-go/sql/v2"
	cefn "github.com/cloudevents/sdk-go/sql/v2/function"
	ceruntime "github.com/cloudevents/sdk-go/sql/v2/runtime"
	cloudevents "github.com/cloudevents/sdk-go/v2"

	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
)

var once = sync.Once{}

// RegisterCESQLVerifyCorrelationIdFilter registers a new function with the CESQL runtime to verify correlation ids
// The function signature is KN_VERIFY_CORRELATION_ID(correlation_id, name, namespace, secret_name, pod_idx, replica_count)
func RegisterCESQLVerifyCorrelationIdFilter(ctx context.Context) error {
	secretInformer := secretinformer.Get(ctx)
	logger := logging.FromContext(ctx)

	var verifyCorrelationId = cefn.NewFunction(
		"KN_VERIFY_CORRELATION_ID",
		[]cesql.Type{cesql.StringType, cesql.StringType, cesql.StringType, cesql.StringType, cesql.IntegerType, cesql.IntegerType},
		nil,
		cesql.BooleanType,
		func(e cloudevents.Event, args []interface{}) (interface{}, error) {
			replyId, name, namespace, secretName, podIdx, replicaCount := args[0].(string), args[1].(string), args[2].(string), args[3].(string), args[4].(int32), args[5].(int32)
			logger.Info("handling CESQL function for KN_VERIFY_CORRELATION_ID")

			parts := strings.Split(replyId, ":")
			if len(parts) != 3 {
				return false, nil
			}

			idx, err := strconv.ParseInt(parts[2], 10, 32)
			if err != nil {
				logger.Warnf("failed to parse correlation id index to int")
				return false, nil
			}
			if int32(idx)%replicaCount != podIdx {
				return false, nil
			}

			secret, err := secretInformer.Lister().Secrets(system.Namespace()).Get(secretName)
			if err != nil {
				logger.Errorf("failed to get secret %s in namespace %s: %s", secretName, namespace, err.Error())
				return false, nil
			}

			for keyName, aesKey := range secret.Data {
				logger.Infof("trying key %s for namespace=%s name=%s", keyName, namespace, name)
				if strings.HasPrefix(keyName, fmt.Sprintf("%s.%s.", namespace, name)) {
					validId, err := VerifyReplyId(replyId, aesKey)
					if err != nil {
						logger.Warnf("failed to verify replyid for event: %s", err)
					}

					logger.Infof("finished key validation: %b", validId)

					if validId {
						return true, nil
					}
				}
			}

			return false, nil
		},
	)

	var err error = nil

	once.Do(func() {
		err = ceruntime.AddFunction(verifyCorrelationId)
	})

	return err
}
