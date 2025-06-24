package requestreply

import (
	"context"
	"sync"

	cesql "github.com/cloudevents/sdk-go/sql/v2"
	cefn "github.com/cloudevents/sdk-go/sql/v2/function"
	ceruntime "github.com/cloudevents/sdk-go/sql/v2/runtime"
	cloudevents "github.com/cloudevents/sdk-go/v2"

	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	"knative.dev/pkg/logging"
)

var once = sync.Once{}

// RegisterCESQLVerifyCorrelationIdFilter registers a new function with the CESQL runtime to verify correlation ids
// The function signature is KN_VERIFY_CORRELATION_ID(correlation_id, namespace, secret_names...)
func RegisterCESQLVerifyCorrelationIdFilter(ctx context.Context) error {
	secretInformer := secretinformer.Get(ctx)
	logger := logging.FromContext(ctx)

	var verifyCorrelationId = cefn.NewFunction(
		"KN_VERIFY_CORRELATION_ID",
		[]cesql.Type{cesql.StringType, cesql.StringType},
		cesql.TypePtr(cesql.StringType),
		cesql.BooleanType,
		func(e cloudevents.Event, args []interface{}) (interface{}, error) {
			replyId, namespace := args[0].(string), args[1].(string)
			secretNames := args[2:]

			for _, secretName := range secretNames {
				secret, err := secretInformer.Lister().Secrets(namespace).Get(secretName.(string))
				if err != nil {
					logger.Warnf("failed to get secret %s in namespace %s: %s", secretName.(string), namespace, err.Error())
					continue
				}

				aesKey, ok := secret.Data["key"]
				if !ok {
					logger.Warnf("no key in secret %s in namespace %s", secretName.(string), namespace)
					continue
				}

				validId, err := VerifyReplyId(replyId, aesKey)
				if err != nil {
					logger.Warnf("failed to verify replyid for event: %s", err)
				}

				if validId {
					return true, nil
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
