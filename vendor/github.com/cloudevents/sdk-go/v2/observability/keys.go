package observability

const (
	// ClientSpanName is the key used to start spans from the client.
	ClientSpanName = "cloudevents.client"

	// metrics/tracing attributes
	SpecversionAttr     = "cloudevents.specversion"
	IdAttr              = "cloudevents.id"
	TypeAttr            = "cloudevents.type"
	SourceAttr          = "cloudevents.source"
	SubjectAttr         = "cloudevents.subject"
	DatacontenttypeAttr = "cloudevents.datacontenttype"
)
