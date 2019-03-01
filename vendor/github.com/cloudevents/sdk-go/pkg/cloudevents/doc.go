/*
Package cloudevents provides primitives to work with CloudEvents specification: https://github.com/cloudevents/spec.


Parsing Event from HTTP Request:
	// req is *http.Request
	event, err := cloudEvents.FromHTTPRequest(req)
	if err != nil {
		panic("Unable to parse event from http Request: " + err.String())
	}


Creating a minimal CloudEvent in version 0.1:
    import "github.com/cloudevents/sdk-go/pkg/cloudevents/v01"
	event := v01.Event{
		EventType:        "com.example.file.created",
		Source:           "/providers/Example.COM/storage/account#fileServices/default/{new-file}",
		EventID:          "ea35b24ede421",
	}


The goal of this package is to provide support for all released versions of CloudEvents, ideally while maintaining
the same API. It will use semantic versioning with following rules:
* MAJOR version increments when backwards incompatible changes is introduced.
* MINOR version increments when backwards compatible feature is introduced INCLUDING support for new CloudEvents version.
* PATCH version increments when a backwards compatible bug fix is introduced.
*/
package cloudevents

/*

New plan.

Everything gets converted into the Canonical form of the event, this
then can select a transport, the transport provides encodings.

At the moment we have cloudevents.[v01, v02]

Canonical form holds an encoded data packet that takes in a provided Codec

Canonical form has two members: Context, and Data


Sending:
cloudevents.[v01, v02] -> { Codec.Encode -> HttpMessage -> Transport[Http] }

Receiving:
{ Transport[Http] -> HttpMessage -> Codec.Decode } -> cloudevents.[v01, v02]

Note: Transport and Codec are grouped.

Transport Codecs supported:
[Binary, Structured, StructuredMirrorHeaders]


## Working with inner data:
cloudevents.[v01, v02].Decode(DataCodec) -> custom data

Working with inner data,

Sending:
cloudevents.[v01, v02].data -> DataCodec.Decode -> custom data

Receiving:
custom data -> DataCodec.Encode -> cloudevents.[v01, v02].data,contentType

Data Codecs supported:
[json, xml, base64, text]


This imples that there is only one canonical form and it evolves and is marked
deprecated as the model evolves.

Spec says:

Event[Context, Data] -> Message

Http Message should have:

Headers
Body
ContentType
ContentLength


*/
