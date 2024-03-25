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
	duckv1 "knative.dev/pkg/apis/duck/v1"
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

// type ResourceInfo interface {
// 	GetNamespace() string
// 	GetName() string
// 	GetTargetInfo() (kind string, name string, namespace string, group string) // target kind and target name
// }

type ResourceInfo interface {
	duckv1.KRShaped
	GetCrossNamespaceRef() duckv1.KReference
}

func CheckNamespace(ctx context.Context, r ResourceInfo, flag *feature.Store) *apis.FieldError {
	if !flag.IsEnabled(feature.CrossNamespaceEventLinks) {
		logging.FromContext(ctx).Debug("Cross-namespace referencing is disabled")
		return nil
	}

	kind := r.GroupVersionKind().Kind
	group := r.GroupVersionKind().Group
	name := r.GetName()
	namespace := r.GetNamespace()
	targetName := r.GetCrossNamespaceRef().Name
	targetNamespace := r.GetCrossNamespaceRef().Namespace
	fieldName := fmt.Sprintf("spec.%sNamespace", kind)

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
		Namespace: targetNamespace,
		Verb:      "get",
		Group:     group,
		Resource:  targetName,
	}

	// Create the SubjectAccessReview
	check := authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			ResourceAttributes: &action,
			User:               userInfo.Username,
			Groups:             userInfo.Groups,
		},
	}

	resp, err := client.AuthorizationV1().SubjectAccessReviews().Create(ctx, &check, metav1.CreateOptions{})

	if err != nil {
		return &apis.FieldError{
			Paths:   []string{fieldName},
			Message: fmt.Sprintf("failed to make authorization request to see if user can get brokers in namespace: %s", err.Error()),
		}
	}

	if !resp.Status.Allowed {
		return &apis.FieldError{
			Paths:   []string{fieldName},
			Message: fmt.Sprintf("user %s is not authorized to get target resource in namespace: %s", userInfo.Username, r.TargetNamespace),
		}
	}

	return nil
}
