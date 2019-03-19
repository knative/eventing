package cloudevents

import (
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"log"
	"mime"
	"strings"
)

// WIP: AS OF FEB 19, 2019

const (
	// CloudEventsVersionV03 represents the version 0.3 of the CloudEvents spec.
	CloudEventsVersionV03 = "0.3"
)

// EventContextV03 represents the non-data attributes of a CloudEvents v0.3
// event.
type EventContextV03 struct {
	// SpecVersion - The version of the CloudEvents specification used by the event.
	SpecVersion string `json:"specversion"`
	// Type - The type of the occurrence which has happened.
	Type string `json:"type"`
	// Source - A URI describing the event producer.
	Source types.URLRef `json:"source"`
	// ID of the event; must be non-empty and unique within the scope of the producer.
	ID string `json:"id"`
	// Time - A Timestamp when the event happened.
	Time *types.Timestamp `json:"time,omitempty"`
	// SchemaURL - A link to the schema that the `data` attribute adheres to.
	SchemaURL *types.URLRef `json:"schemaurl,omitempty"`
	// GetDataMediaType - A MIME (RFC2046) string describing the media type of `data`.
	// TODO: Should an empty string assume `application/json`, `application/octet-stream`, or auto-detect the content?
	DataContentType *string `json:"datacontenttype,omitempty"`
	// Extensions - Additional extension metadata beyond the base spec.
	Extensions map[string]interface{} `json:"-,omitempty"` // TODO: decide how we want extensions to be inserted
}

var _ EventContext = (*EventContextV03)(nil)

func (ec EventContextV03) GetSpecVersion() string {
	if ec.SpecVersion != "" {
		return ec.SpecVersion
	}
	return CloudEventsVersionV03
}

func (ec EventContextV03) GetDataContentType() string {
	if ec.DataContentType != nil {
		return *ec.DataContentType
	}
	return ""
}

func (ec EventContextV03) GetDataMediaType() string {
	if ec.DataContentType != nil {
		mediaType, _, err := mime.ParseMediaType(*ec.DataContentType)
		if err != nil {
			log.Printf("failed to parse media type from DataContentType: %s", err)
			return ""
		}
		return mediaType
	}
	return ""
}
func (ec EventContextV03) GetType() string {
	return ec.Type
}

func (ec EventContextV03) AsV01() EventContextV01 {
	ecv2 := ec.AsV02()
	return ecv2.AsV01()
}

func (ec EventContextV03) AsV02() EventContextV02 {
	ret := EventContextV02{
		SpecVersion: CloudEventsVersionV02,
		ID:          ec.ID,
		Time:        ec.Time,
		Type:        ec.Type,
		SchemaURL:   ec.SchemaURL,
		ContentType: ec.DataContentType,
		Source:      ec.Source,
		Extensions:  ec.Extensions,
	}
	return ret
}

func (ec EventContextV03) AsV03() EventContextV03 {
	ec.SpecVersion = CloudEventsVersionV03
	return ec
}

// Validate returns errors based on requirements from the CloudEvents spec.
// For more details, see https://github.com/cloudevents/spec/blob/master/spec.md
// As of Feb 26, 2019, commit 17c32ea26baf7714ad027d9917d03d2fff79fc7e
func (ec EventContextV03) Validate() error {
	errors := []string(nil)

	// type
	// Type: String
	// Constraints:
	//  REQUIRED
	//  MUST be a non-empty string
	//  SHOULD be prefixed with a reverse-DNS name. The prefixed domain dictates the organization which defines the semantics of this event type.
	eventType := strings.TrimSpace(ec.Type)
	if eventType == "" {
		errors = append(errors, "type: MUST be a non-empty string")
	}

	// specversion
	// Type: String
	// Constraints:
	//  REQUIRED
	//  MUST be a non-empty string
	specVersion := strings.TrimSpace(ec.SpecVersion)
	if specVersion == "" {
		errors = append(errors, "specversion: MUST be a non-empty string")
	}

	// source
	// Type: URI-reference
	// Constraints:
	//  REQUIRED
	source := strings.TrimSpace(ec.Source.String())
	if source == "" {
		errors = append(errors, "source: REQUIRED")
	}

	// id
	// Type: String
	// Constraints:
	//  REQUIRED
	//  MUST be a non-empty string
	//  MUST be unique within the scope of the producer
	id := strings.TrimSpace(ec.ID)
	if id == "" {
		errors = append(errors, "id: MUST be a non-empty string")

		// no way to test "MUST be unique within the scope of the producer"
	}

	// time
	// Type: Timestamp
	// Constraints:
	//  OPTIONAL
	//  If present, MUST adhere to the format specified in RFC 3339
	// --> no need to test this, no way to set the time without it being valid.

	// schemaurl
	// Type: URI
	// Constraints:
	//  OPTIONAL
	//  If present, MUST adhere to the format specified in RFC 3986
	if ec.SchemaURL != nil {
		schemaURL := strings.TrimSpace(ec.SchemaURL.String())
		// empty string is not RFC 3986 compatible.
		if schemaURL == "" {
			errors = append(errors, "schemaurl: if present, MUST adhere to the format specified in RFC 3986")
		}
	}

	// datacontenttype
	// Type: String per RFC 2046
	// Constraints:
	//  OPTIONAL
	//  If present, MUST adhere to the format specified in RFC 2046
	if ec.DataContentType != nil {
		dataContentType := strings.TrimSpace(*ec.DataContentType)
		if dataContentType == "" {
			// TODO: need to test for RFC 2046
			errors = append(errors, "datacontenttype: if present, MUST adhere to the format specified in RFC 2046")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}
	return nil
}
