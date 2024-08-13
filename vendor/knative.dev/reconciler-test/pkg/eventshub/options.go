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

package eventshub

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/network"

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub/dropevents"
	"knative.dev/reconciler-test/pkg/k8s"
)

type forwarderKey struct{}

// WithKnativeServiceForwarder deploys a Knative Service forwarder that will forward requests to eventshub.
func WithKnativeServiceForwarder(ctx context.Context, env environment.Environment) (context.Context, error) {
	return context.WithValue(ctx, forwarderKey{}, true), nil
}

func isForwarder(ctx context.Context) bool {
	v := ctx.Value(forwarderKey{})
	return v != nil && v.(bool)
}

// EventsHubOption is used to define an env for the eventshub image
type EventsHubOption = func(context.Context, map[string]string) error

// StartReceiver starts the receiver in the eventshub
// This can be used together with EchoEvent, ReplyWithTransformedEvent, ReplyWithAppendedData
var StartReceiver EventsHubOption = envAdditive(EventGeneratorsEnv, "receiver")

// StartReceiverTLS starts the receiver in the eventshub with TLS enforcement.
// This can be used together with EchoEvent, ReplyWithTransformedEvent, ReplyWithAppendedData.
//
// It requires cert-manager operator to be able to create TLS Certificate.
// To get the CA certificate used you can use GetCaCerts.
var StartReceiverTLS EventsHubOption = compose(StartReceiver, envAdditive(EnforceTLS, "true"))

// StartSender starts the sender in the eventshub
// This can be used together with InputEvent, AddTracing, EnableIncrementalId, InputEncoding and InputHeader options
func StartSender(sinkSvc string) EventsHubOption {
	return func(ctx context.Context, m map[string]string) error {
		return StartSenderURL("http://"+network.GetServiceHostname(sinkSvc, environment.FromContext(ctx).Namespace()))(ctx, m)
	}
}

// StartSenderTLS starts the sender in the eventshub with TLS enforcement.
func StartSenderTLS(sinkSvc string, caCerts *string) EventsHubOption {
	return func(ctx context.Context, envs map[string]string) error {
		return StartSenderURLTLS(
			"https://"+network.GetServiceHostname(sinkSvc, environment.FromContext(ctx).Namespace()),
			caCerts,
		)(ctx, envs)
	}
}

// StartSenderToResource starts the sender in the eventshub pointing to the provided resource
// This can be used together with InputEvent, AddTracing, EnableIncrementalId, InputEncoding and InputHeader options
func StartSenderToResource(gvr schema.GroupVersionResource, name string) EventsHubOption {
	return func(ctx context.Context, envs map[string]string) error {
		env := environment.FromContext(ctx)
		return StartSenderToNamespacedResource(gvr, name, env.Namespace())(ctx, envs)
	}
}

// StartSenderToNamespacedResource starts the sender in the eventshub pointing to the provided resource
// This can be used together with InputEvent, AddTracing, EnableIncrementalId, InputEncoding and InputHeader options
func StartSenderToNamespacedResource(gvr schema.GroupVersionResource, name, namespace string) EventsHubOption {
	return func(ctx context.Context, envs map[string]string) error {
		u, err := k8s.NamespacedAddress(ctx, gvr, name, namespace)
		if err != nil {
			return err
		}
		if u == nil {
			return fmt.Errorf("resource %v named %s is not addressable", gvr, name)
		}

		if u.URL.Scheme == "https" {
			return compose(StartSenderURLTLS(u.URL.String(), u.CACerts), oidcSinkAudience(u.Audience))(ctx, envs)
		}

		return compose(StartSenderURL(u.URL.String()), oidcSinkAudience(u.Audience))(ctx, envs)
	}
}

// StartSenderToResourceTLS starts the sender in the eventshub pointing to the provided resource.
// `caCerts` parameter is optional, if nil, it will fall back to use the addressable CA certs.
// This can be used together with InputEvent, AddTracing, EnableIncrementalId, InputEncoding and InputHeader options
func StartSenderToResourceTLS(gvr schema.GroupVersionResource, name string, caCerts *string) EventsHubOption {
	return func(ctx context.Context, m map[string]string) error {
		env := environment.FromContext(ctx)
		return StartSenderToNamespacedResourceTLS(gvr, name, env.Namespace(), caCerts)(ctx, m)
	}
}

// StartSenderToNamespacedResourceTLS starts the sender in the eventshub pointing to the provided namespaced resource.
// `caCerts` parameter is optional, if nil, it will fall back to use the addressable CA certs.
// This can be used together with InputEvent, AddTracing, EnableIncrementalId, InputEncoding and InputHeader options
func StartSenderToNamespacedResourceTLS(gvr schema.GroupVersionResource, name, namespace string, caCerts *string) EventsHubOption {
	return func(ctx context.Context, m map[string]string) error {
		u, err := k8s.NamespacedAddress(ctx, gvr, name, namespace)
		if err != nil {
			return err
		}
		if u == nil {
			return fmt.Errorf("resource %v named %s is not addressable", gvr, name)
		}
		u.URL.Scheme = "https"

		if caCerts == nil && u.CACerts != nil {
			caCerts = u.CACerts
		}

		return compose(StartSenderURLTLS(u.URL.String(), caCerts), oidcSinkAudience(u.Audience))(ctx, m)
	}
}

// StartSenderURL starts the sender in the eventshub sinking to a URL.
// This can be used together with InputEvent, AddTracing, EnableIncrementalId, InputEncoding and InputHeader options
func StartSenderURL(sink string) EventsHubOption {
	return compose(envAdditive(EventGeneratorsEnv, "sender"), func(ctx context.Context, envs map[string]string) error {
		envs["SINK"] = sink
		return nil
	})
}

// StartSenderURLTLS starts the sender in the eventshub sinking to a URL.
// This can be used together with InputEvent, AddTracing, EnableIncrementalId, InputEncoding and InputHeader options
func StartSenderURLTLS(sink string, caCerts *string) EventsHubOption {
	return compose(envAdditive(EventGeneratorsEnv, "sender"), envAdditive(EnforceTLS, "true"), envCACerts(caCerts),
		func(ctx context.Context, envs map[string]string) error {
			envs["SINK"] = sink
			return nil
		})
}

func IssuerRef(kind, name string) EventsHubOption {
	return compose(
		envAdditive(tlsIssuerKind, kind),
		envAdditive(tlsIssuerName, name),
	)
}

// --- Receiver options

// EchoEvent is an option to let the eventshub reply with the received event
var EchoEvent EventsHubOption = envOption("REPLY", "true")

// ReplyWithTransformedEvent is an option to let the eventshub reply with the transformed event
func ReplyWithTransformedEvent(replyEventType string, replyEventSource string, replyEventData string) EventsHubOption {
	return compose(
		envOption("REPLY", "true"),
		envOptionalOpt("REPLY_EVENT_TYPE", replyEventType),
		envOptionalOpt("REPLY_EVENT_SOURCE", replyEventSource),
		envOptionalOpt("REPLY_EVENT_DATA", replyEventData),
	)
}

// ReplyWithAppendedData is an option to let the eventshub reply with the transformed event with appended data
func ReplyWithAppendedData(appendData string) EventsHubOption {
	return compose(
		envOption("REPLY", "true"),
		envOptionalOpt("REPLY_APPEND_DATA", appendData),
	)
}

// ResponseWaitTime defines how much the receiver has to wait before replying.
func ResponseWaitTime(delay time.Duration) EventsHubOption {
	return envDuration("RESPONSE_WAIT_TIME", delay)
}

// FibonacciDrop will cause the receiver to reply with a bad status code following the fibonacci sequence
var FibonacciDrop = envOption("SKIP_ALGORITHM", dropevents.Fibonacci)

// DropFirstN will cause the receiver to reply with a bad status code to the first n events
func DropFirstN(n uint) EventsHubOption {
	return compose(
		envOption("SKIP_ALGORITHM", dropevents.Sequence),
		envOption("SKIP_COUNTER", strconv.FormatUint(uint64(n), 10)),
	)
}

// DropEventsResponseCode will cause the receiver to reply with the specific status code to the dropped events
func DropEventsResponseCode(code int) EventsHubOption {
	return compose(
		envOption("SKIP_RESPONSE_CODE", strconv.Itoa(code)),
	)
}

// DropEventsResponseBody will cause the receiver to reply with the specific body to the dropped events
func DropEventsResponseBody(body string) EventsHubOption {
	return envOption("SKIP_RESPONSE_BODY", body)
}

// DropEventsResponseHeaders will cause the receiver to reply with the specific headers to the dropped events
func DropEventsResponseHeaders(headers map[string]string) EventsHubOption {
	headerEnvConfigString := ""
	for k, v := range headers {
		if headerEnvConfigString != "" {
			headerEnvConfigString = headerEnvConfigString + ","
		}
		headerEnvConfigString = fmt.Sprintf("%s%s:%s", headerEnvConfigString, k, v) // Format as envconfig map[string]string
	}
	return compose(
		envOptionalOpt("SKIP_RESPONSE_HEADERS", headerEnvConfigString),
	)
}

// OIDCReceiverAudience sets the expected audience for received OIDC tokens on the receiver side
func OIDCReceiverAudience(aud string) EventsHubOption {
	return compose(envOption(OIDCReceiverAudienceEnv, aud), envOIDCEnabled())
}

// VerifyEventFormat has the receiver verify the event format when receiving events
func VerifyEventFormat(format string) EventsHubOption {
	return compose(envOption("EVENT_FORMAT", format))
}

// VerifyEventFormat has the receiver verify the event format is structured when receiving events
func VerifyEventFormatStructured() EventsHubOption {
	return VerifyEventFormat("json")
}

// VerifyEventFormat has the receiver verify the event format is binary when receiving events
func VerifyEventFormatBinary() EventsHubOption {
	return VerifyEventFormat("binary")
}

// --- Sender options

// InitialSenderDelay defines how much the sender has to wait (in millisecond), when started, before start sending events.
// Note: this delay is executed before the probe sink.
func InitialSenderDelay(delay time.Duration) EventsHubOption {
	return envDuration("DELAY", delay)
}

// EnableProbeSink probes the sink with HTTP head requests up until the sink replies.
// The specified duration defines the maximum timeout to probe it, before failing.
// Note: the probe sink is executed after the initial delay
func EnableProbeSink(timeout time.Duration) EventsHubOption {
	return compose(
		envOption("PROBE_SINK", "true"),
		envDuration("PROBE_SINK_TIMEOUT", timeout),
	)
}

// DisableProbeSink will disable the probe sink feature of sender, starting sending directly events after it's started.
var DisableProbeSink = envOption("PROBE_SINK", "false")

// InputYAML is an option to provide the events to send via yaml path when deploying the event sender
func InputYAML(path string) EventsHubOption {
	return envAdditive("INPUT_YAML", path)
}

// InputEvent is an option to provide the event to send when deploying the event sender
func InputEvent(event cloudevents.Event) EventsHubOption {
	encodedEvent, err := json.Marshal(event)
	if err != nil {
		return func(ctx context.Context, envs map[string]string) error {
			return err
		}
	}
	return envOption("INPUT_EVENT", string(encodedEvent))
}

// InputEventWithEncoding is an option to provide the event to send when deploying the event sender forcing the specified encoding.
func InputEventWithEncoding(event cloudevents.Event, encoding cloudevents.Encoding) EventsHubOption {
	encodedEvent, err := json.Marshal(event)
	if err != nil {
		return func(ctx context.Context, envs map[string]string) error {
			return err
		}
	}
	return compose(
		envOption("INPUT_EVENT", string(encodedEvent)),
		envOption("EVENT_ENCODING", encoding.String()),
	)
}

// InputHeader adds the following header to the sent headers.
func InputHeader(k, v string) EventsHubOption {
	return envAdditive("INPUT_HEADERS", k+":"+v)
}

// InputBody overwrites the request header with the following body.
func InputBody(b string) EventsHubOption {
	return envOption("INPUT_BODY", b)
}

// InputMethod overrides which http method to use when sending events (default is POST)
func InputMethod(method string) EventsHubOption {
	return envOption("INPUT_METHOD", method)
}

// OIDCExpiredToken adds an expired OIDC token to the request. As the minimal
// expiry for JWTs from Kubernetes are 10 minutes, the sender will delay the
// send by 10 + 1 minutes.
// This should be used in combination of an increase of the poll timout (via
// environment.PollTimingsFromContext()) to not run in the default 2 minutes
// timeout while waiting for an event which is send after 10 + 1 minutes.
func OIDCExpiredToken() EventsHubOption {
	return compose(envOption(OIDCGenerateExpiredTokenEnv, "true"), InitialSenderDelay(time.Minute*(OIDCTokenExpiryMinutes+1)), envOIDCEnabled())
}

// OIDCInvalidAudience creates an OIDC token with an invalid audience
func OIDCInvalidAudience() EventsHubOption {
	return compose(envOption(OIDCGenerateInvalidAudienceTokenEnv, "true"), envOIDCEnabled())
}

// OIDCSinkAudience sets the Audience of the Sink
func OIDCSinkAudience(aud string) EventsHubOption {
	return oidcSinkAudience(&aud)
}

func oidcSinkAudience(aud *string) EventsHubOption {
	if aud != nil && *aud != "" {
		// if the sink has an audience set, we enable OIDC to get a token added
		return compose(envOption(OIDCSinkAudienceEnv, *aud), envOIDCEnabled())
	}

	return noop
}

// OIDCCorruptedSignature adds an OIDC token with an invalid signature to the request.
func OIDCCorruptedSignature() EventsHubOption {
	return compose(envOption(OIDCGenerateCorruptedSignatureTokenEnv, "true"), envOIDCEnabled())
}

// OIDCSubject sets the name of the OIDC subject to use by the sender. If this option is not set, it defaults to "oidc-<eventshub-name>"
func OIDCSubject(sub string) EventsHubOption {
	return compose(envOption(OIDCSubjectEnv, sub), envOIDCEnabled())
}

// OIDCToken adds the given token used for OIDC authentication to the request.
func OIDCToken(jwt string) EventsHubOption {
	return compose(envOption(OIDCTokenEnv, jwt), envOIDCEnabled())
}

// AddTracing adds tracing headers when sending events.
// Deprecated: Exporting traces from the client/sender is enabled by default.
var AddTracing = envOption("ADD_TRACING", "true")

// AddSequence adds an extension named 'sequence' which contains the incremental number of sent events
// (similar to EnableIncrementalId, but without overwriting the id attribute).
var AddSequence = envOption("ADD_SEQUENCE", "true")

// EnableIncrementalId replaces the event id with a new incremental id for each sent event.
var EnableIncrementalId = envOption("INCREMENTAL_ID", "true")

// OverrideTime overrides the event time with the time when sending the event.
var OverrideTime = envOption("OVERRIDE_TIME", "true")

// SendMultipleEvents defines how much events to send and the period (in millisecond) between them.
func SendMultipleEvents(numberOfEvents int, period time.Duration) EventsHubOption {
	return compose(
		envOption("MAX_MESSAGES", strconv.Itoa(numberOfEvents)),
		envDuration("PERIOD", period),
	)
}

// --- Options utils

func noop(context.Context, map[string]string) error {
	return nil
}

func compose(options ...EventsHubOption) EventsHubOption {
	return func(ctx context.Context, envs map[string]string) error {
		for _, opt := range options {
			if err := opt(ctx, envs); err != nil {
				return err
			}
		}
		return nil
	}
}

func envOptionalOpt(key, value string) EventsHubOption {
	if value != "" {
		return func(ctx context.Context, envs map[string]string) error {
			envs[key] = value
			return nil
		}
	} else {
		return noop
	}
}

func envOption(key, value string) EventsHubOption {
	return func(ctx context.Context, envs map[string]string) error {
		envs[key] = value
		return nil
	}
}

func envAdditive(key, value string) EventsHubOption {
	return func(ctx context.Context, m map[string]string) error {
		if containedValue, ok := m[key]; ok {
			m[key] = containedValue + "," + value
		} else {
			m[key] = value
		}
		return nil
	}
}

func envDuration(key string, value time.Duration) EventsHubOption {
	return envOption(key, strconv.Itoa(int(math.Ceil(value.Seconds()))))
}

func envCACerts(caCerts *string) EventsHubOption {
	return func(ctx context.Context, m map[string]string) error {
		if caCerts != nil {
			return envAdditive("CA_CERTS", *caCerts)(ctx, m)
		}
		return nil
	}
}

func envOIDCEnabled() EventsHubOption {
	return envOption(OIDCEnabledEnv, "true")
}
