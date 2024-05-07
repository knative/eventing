/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package assert

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/reconciler-test/pkg/environment"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"

	"knative.dev/reconciler-test/pkg/eventshub"
)

// Matcher that never fails
func Any() eventshub.EventInfoMatcher {
	return func(ei eventshub.EventInfo) error {
		return nil
	}
}

// Matcher that fails if there is an error in the EventInfo
func NoError() eventshub.EventInfoMatcher {
	return func(ei eventshub.EventInfo) error {
		if ei.Error != "" {
			return fmt.Errorf("not expecting an error in event info: %s", ei.Error)
		}
		return nil
	}
}

// Convert a matcher that checks valid messages to a function
// that checks EventInfo structures, returning an error for any that don't
// contain valid events.
func MatchEvent(evf ...cetest.EventMatcher) eventshub.EventInfoMatcher {
	return func(ei eventshub.EventInfo) error {
		if ei.Event == nil {
			return fmt.Errorf("Saw nil event")
		} else {
			return cetest.AllOf(evf...)(*ei.Event)
		}
	}
}

// Convert a matcher that checks valid messages to a function
// that checks EventInfo structures, returning an error for any that don't
// contain valid events.
func HasAdditionalHeader(key, value string) eventshub.EventInfoMatcher {
	key = strings.ToLower(key)
	return func(ei eventshub.EventInfo) error {
		for k, v := range ei.HTTPHeaders {
			if strings.ToLower(k) == key && v[0] == value {
				return nil
			}
		}
		return fmt.Errorf("cannot find header '%s' = '%s' between the headers", key, value)
	}
}

// Reexport kinds here to simplify the usage
const (
	EventReceived = eventshub.EventReceived
	EventRejected = eventshub.EventRejected

	EventSent     = eventshub.EventSent
	EventResponse = eventshub.EventResponse
)

// MatchKind matches the kind of EventInfo
func MatchKind(kind eventshub.EventKind) eventshub.EventInfoMatcher {
	return func(info eventshub.EventInfo) error {
		if kind != info.Kind {
			return fmt.Errorf("event kind don't match. Expected: '%s', Actual: '%s'", kind, info.Kind)
		}
		return nil
	}
}

func OneOf(matchers ...eventshub.EventInfoMatcher) eventshub.EventInfoMatcher {
	return func(info eventshub.EventInfo) error {
		var lastErr error
		for _, m := range matchers {
			err := m(info)
			if err == nil {
				return nil
			}
			lastErr = err
		}
		return lastErr
	}
}

// MatchStatusCode matches the status code of EventInfo
func MatchStatusCode(statusCode int) eventshub.EventInfoMatcher {
	return func(info eventshub.EventInfo) error {
		if info.StatusCode != statusCode {
			return fmt.Errorf("event status code don't match. Expected: '%d', Actual: '%d'", statusCode, info.StatusCode)
		}
		return nil
	}
}

// MatchOIDCUser matches the OIDC username used for the request
func MatchOIDCUser(username string) eventshub.EventInfoMatcher {
	return func(info eventshub.EventInfo) error {
		if info.OIDCUserInfo == nil {
			return fmt.Errorf("event OIDC usernames don't match: Expected %q, but no OIDC user info in the event", username)
		}
		if info.OIDCUserInfo.Username != username {
			return fmt.Errorf("event OIDC usernames don't match. Expected: %q, Actual: %q", username, info.OIDCUserInfo.Username)
		}

		return nil
	}
}

// MatchOIDCUserFromResource matches the given resources OIDC identifier
func MatchOIDCUserFromResource(gvr schema.GroupVersionResource, resourceName string) eventshub.EventInfoMatcherCtx {

	type AuthenticatableType struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata,omitempty"`

		Status struct {
			Auth *duckv1.AuthStatus `json:"auth,omitempty"`
		} `json:"status"`
	}

	return func(ctx context.Context, info eventshub.EventInfo) error {

		env := environment.FromContext(ctx)

		us, err := dynamicclient.Get(ctx).Resource(gvr).Namespace(env.Namespace()).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error getting resource: %w", err)
		}

		obj := &AuthenticatableType{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, obj); err != nil {
			return fmt.Errorf("error from DefaultUnstructured.Dynamiconverter: %w", err)
		}

		if obj.Status.Auth == nil || (obj.Status.Auth.ServiceAccountName == nil && len(obj.Status.Auth.ServiceAccountNames) == 0) {
			return fmt.Errorf("resource does not have an OIDC service account set")
		}

		objSAs := obj.Status.Auth.ServiceAccountNames
		if obj.Status.Auth.ServiceAccountName != nil {
			objSAs = append(objSAs, *obj.Status.Auth.ServiceAccountName)
		}

		for _, sa := range objSAs {
			objFullSAName := fmt.Sprintf("system:serviceaccount:%s:%s", obj.GetNamespace(), sa)

			if objFullSAName == info.OIDCUserInfo.Username {
				// found
				return nil
			}
		}

		return fmt.Errorf("OIDC identity in event does not match identity/identities of resource. Event: %q, resource: %q", info.OIDCUserInfo.Username, strings.Join(objSAs, ", "))
	}
}

// MatchHeartBeatsImageMessage matches that the data field of the event, in the format of the heartbeats image, contains the following msg field
func MatchHeartBeatsImageMessage(expectedMsg string) cetest.EventMatcher {
	return cetest.AllOf(
		cetest.HasDataContentType(cloudevents.ApplicationJSON),
		func(have cloudevents.Event) error {
			var m map[string]interface{}
			err := have.DataAs(&m)
			if err != nil {
				return fmt.Errorf("cannot parse heartbeats message %s", err.Error())
			}
			if m["msg"].(string) != expectedMsg {
				return fmt.Errorf("heartbeats message don't match. Expected: '%s', Actual: '%s'", expectedMsg, m["msg"].(string))
			}
			return nil
		},
	)
}
