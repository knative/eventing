package cloudevents

// Package cloudevents alias' common functions and types to improve discoverability and reduce
// the number of imports for simple HTTP clients.

import (
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/context"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/observability"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/transport/http"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/types"
)

// Client

// Deprecated: legacy.
type ClientOption client.Option

// Deprecated: legacy.
type Client = client.Client

// Deprecated: legacy.
type ConvertFn = client.ConvertFn

// Event

// Deprecated: legacy.
type Event = cloudevents.Event

// Deprecated: legacy.
type EventResponse = cloudevents.EventResponse

// Context

// Deprecated: legacy.
type EventContext = cloudevents.EventContext

// Deprecated: legacy.
type EventContextV1 = cloudevents.EventContextV1

// Deprecated: legacy.
type EventContextV01 = cloudevents.EventContextV01

// Deprecated: legacy.
type EventContextV02 = cloudevents.EventContextV02

// Deprecated: legacy.
type EventContextV03 = cloudevents.EventContextV03

// Custom Types

// Deprecated: legacy.
type Timestamp = types.Timestamp

// Deprecated: legacy.
type URLRef = types.URLRef

// HTTP Transport

// Deprecated: legacy.
type HTTPOption http.Option

// Deprecated: legacy.
type HTTPTransport = http.Transport

// Deprecated: legacy.
type HTTPTransportContext = http.TransportContext

// Deprecated: legacy.
type HTTPTransportResponseContext = http.TransportResponseContext

// Deprecated: legacy.
type HTTPEncoding = http.Encoding

const (
	// Encoding
	// Deprecated: legacy.
	ApplicationXML = cloudevents.ApplicationXML
	// Deprecated: legacy.
	ApplicationJSON = cloudevents.ApplicationJSON
	// Deprecated: legacy.
	ApplicationCloudEventsJSON = cloudevents.ApplicationCloudEventsJSON
	// Deprecated: legacy.
	ApplicationCloudEventsBatchJSON = cloudevents.ApplicationCloudEventsBatchJSON
	// Deprecated: legacy.
	Base64 = cloudevents.Base64

	// Event Versions

	// Deprecated: legacy.
	VersionV1 = cloudevents.CloudEventsVersionV1
	// Deprecated: legacy.
	VersionV01 = cloudevents.CloudEventsVersionV01
	// Deprecated: legacy.
	VersionV02 = cloudevents.CloudEventsVersionV02
	// Deprecated: legacy.
	VersionV03 = cloudevents.CloudEventsVersionV03

	// HTTP Transport Encodings

	// Deprecated: legacy.
	HTTPBinaryV1 = http.BinaryV1
	// Deprecated: legacy.
	HTTPStructuredV1 = http.StructuredV1
	// Deprecated: legacy.
	HTTPBatchedV1 = http.BatchedV1
	// Deprecated: legacy.
	HTTPBinaryV01 = http.BinaryV01
	// Deprecated: legacy.
	HTTPStructuredV01 = http.StructuredV01
	// Deprecated: legacy.
	HTTPBinaryV02 = http.BinaryV02
	// Deprecated: legacy.
	HTTPStructuredV02 = http.StructuredV02
	// Deprecated: legacy.
	HTTPBinaryV03 = http.BinaryV03
	// Deprecated: legacy.
	HTTPStructuredV03 = http.StructuredV03
	// Deprecated: legacy.
	HTTPBatchedV03 = http.BatchedV03

	// Context HTTP Transport Encodings

	Binary     = http.Binary
	Structured = http.Structured
)

var (
	// ContentType Helpers

	// Deprecated: legacy.
	StringOfApplicationJSON = cloudevents.StringOfApplicationJSON
	// Deprecated: legacy.
	StringOfApplicationXML = cloudevents.StringOfApplicationXML
	// Deprecated: legacy.
	StringOfApplicationCloudEventsJSON = cloudevents.StringOfApplicationCloudEventsJSON
	// Deprecated: legacy.
	StringOfApplicationCloudEventsBatchJSON = cloudevents.StringOfApplicationCloudEventsBatchJSON
	// Deprecated: legacy.
	StringOfBase64 = cloudevents.StringOfBase64

	// Client Creation

	// Deprecated: legacy.
	NewClient = client.New
	// Deprecated: legacy.
	NewDefaultClient = client.NewDefault

	// Client Options

	// Deprecated: legacy.
	WithEventDefaulter = client.WithEventDefaulter
	// Deprecated: legacy.
	WithUUIDs = client.WithUUIDs
	// Deprecated: legacy.
	WithTimeNow = client.WithTimeNow
	// Deprecated: legacy.
	WithConverterFn = client.WithConverterFn
	// Deprecated: legacy.
	WithDataContentType = client.WithDataContentType
	// Deprecated: legacy.
	WithoutTracePropagation = client.WithoutTracePropagation

	// Event Creation

	// Deprecated: legacy.
	NewEvent = cloudevents.New

	// Tracing

	// Deprecated: legacy.
	EnableTracing = observability.EnableTracing

	// Context

	// Deprecated: legacy.
	ContextWithTarget = context.WithTarget
	// Deprecated: legacy.
	TargetFromContext = context.TargetFrom
	// Deprecated: legacy.
	ContextWithEncoding = context.WithEncoding
	// Deprecated: legacy.
	EncodingFromContext = context.EncodingFrom

	// Custom Types

	// Deprecated: legacy.
	ParseTimestamp = types.ParseTimestamp
	// Deprecated: legacy.
	ParseURLRef = types.ParseURLRef
	// Deprecated: legacy.
	ParseURIRef = types.ParseURIRef
	// Deprecated: legacy.
	ParseURI = types.ParseURI

	// HTTP Transport

	// Deprecated: legacy.
	NewHTTPTransport = http.New

	// HTTP Transport Options

	// Deprecated: legacy.
	WithTarget = http.WithTarget
	// Deprecated: legacy.
	WithMethod = http.WithMethod
	// Deprecated: legacy.
	WitHHeader = http.WithHeader
	// Deprecated: legacy.
	WithShutdownTimeout = http.WithShutdownTimeout
	// Deprecated: legacy.
	WithEncoding = http.WithEncoding
	// Deprecated: legacy.
	WithContextBasedEncoding = http.WithContextBasedEncoding
	// Deprecated: legacy.
	WithBinaryEncoding = http.WithBinaryEncoding
	// Deprecated: legacy.
	WithStructuredEncoding = http.WithStructuredEncoding
	// Deprecated: legacy.
	WithPort = http.WithPort
	// Deprecated: legacy.
	WithPath = http.WithPath
	// Deprecated: legacy.
	WithMiddleware = http.WithMiddleware
	// Deprecated: legacy.
	WithLongPollTarget = http.WithLongPollTarget
	// Deprecated: legacy.
	WithListener = http.WithListener
	// Deprecated: legacy.
	WithHTTPTransport = http.WithHTTPTransport

	// HTTP Context

	// Deprecated: legacy.
	HTTPTransportContextFrom = http.TransportContextFrom
	// Deprecated: legacy.
	ContextWithHeader = http.ContextWithHeader
	// Deprecated: legacy.
	SetContextHeaders = http.SetContextHeaders
)
