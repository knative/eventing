package crossnamespace

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// type ResourceInfo struct {
// 	Kind            string
// 	Name            string
// 	Namespace       string
// 	Target          string
// 	TargetName      string
// 	TargetNamespace string
// 	TargetGroup     string
// 	FeatureStore    *feature.Store
// }

type ResourceInfo interface {
	GetNamespace() string
	GetName() string
	GetTargetInfo() (kind string, name string, namespace string, group string) // target kind and target name
}

func CheckNamespace(ctx context.Context, r Resource, flag *feature.Store) *apis.FieldError {
	if !flag.IsEnabled(feature.CrossNamespaceEventLinks) {
		logging.FromContext(ctx).Debug("Cross-namespace referencing is disabled")
		return nil
	}

	t_kind, t_name, t_namespace, t_group := r.GetTargetInfo()
	fieldName := fmt.Sprintf("spec.%sNamespace", t_namespace)

	// GetUserInfo accesses the UserInfo attached to the webhook context.
	userInfo := apis.GetUserInfo(ctx)
	if userInfo == nil {
		return &apis.FieldError{
			Paths:   []string{fieldName},
			Message: "failed to get userInfo, which is needed to validate access to the target namespace",
		}
	}

	// GetConfig gets the current config from the context.
	config := injection.GetConfig(ctx)
	logging.FromContext(ctx).Info("got config", zap.Any("config", config))
	if config == nil {
		return &apis.FieldError{
			Paths:   []string{fieldName},
			Message: "failed to get config, which is needed to validate the resources created with the namespace different than the target's namespace",
		}
	}

	// NewForConfig creates a new Clientset for the given config.
	// If config's RateLimiter is not set and QPS and Burst are acceptable,
	// NewForConfig will generate a rate-limiter in configShallowCopy.
	// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
	// where httpClient was generated with rest.HTTPClientFor(c).
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return &apis.FieldError{
			Paths:   []string{fieldName},
			Message: "failed to get k8s client, which is needed to validate the resources created with the namespace different than the target's namespace",
		}
	}

	// SubjectAccessReview checks if the user is authorized to perform an action.
	// Define the action
	action := authv1.ResourceAttributes{
		Namespace: t_namespace,
		Verb:      "get",
		Group:     t_group,
		Resource:  t_name,
	}

	// Create the SubjectAccessReview
	check := authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			ResourceAttributes: &action,
			User:               userInfo.Username,
			Groups:             userInfo.Groups,
		},
	}

	// Make the request to the server.
	resp, err := client.AuthorizationV1().SubjectAccessReviews().Create(ctx, &check, metav1.CreateOptions{})

	// If the request fails, return an error.
	if err != nil {
		return &apis.FieldError{
			Paths:   []string{fieldName},
			Message: fmt.Sprintf("failed to make authorization request to see if user can get brokers in namespace: %s", err.Error()),
		}
	}

	// If the request is not allowed, return an error.
	if !resp.Status.Allowed {
		return &apis.FieldError{
			Paths:   []string{fieldName},
			Message: fmt.Sprintf("user %s is not authorized to get target resource in namespace: %s", userInfo.Username, r.TargetNamespace),
		}
	}

	return nil
}
