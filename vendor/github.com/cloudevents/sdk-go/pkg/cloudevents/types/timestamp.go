package types

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"time"
)

type Timestamp struct {
	time.Time
}

func ParseTimestamp(t string) *Timestamp {
	if t == "" {
		return nil
	}
	timestamp, err := time.Parse(time.RFC3339Nano, t)
	if err != nil {
		return nil
	}
	return &Timestamp{Time: timestamp}
}

// This allows json marshaling to always be in RFC3339Nano format.
func (t *Timestamp) MarshalJSON() ([]byte, error) {
	if t == nil || t.IsZero() {
		return []byte(`""`), nil
	}
	rfc3339 := fmt.Sprintf("%q", t.UTC().Format(time.RFC3339Nano))
	return []byte(rfc3339), nil
}

func (t *Timestamp) UnmarshalJSON(b []byte) error {
	var timestamp string
	if err := json.Unmarshal(b, &timestamp); err != nil {
		return err
	}
	if pt := ParseTimestamp(timestamp); pt != nil {
		*t = *pt
	}
	return nil
}

func (t *Timestamp) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if t == nil || t.IsZero() {
		return e.EncodeElement(nil, start)
	}
	v := t.UTC().Format(time.RFC3339Nano)
	return e.EncodeElement(v, start)
}

func (t *Timestamp) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var timestamp string
	if err := d.DecodeElement(&timestamp, &start); err != nil {
		return err
	}
	if pt := ParseTimestamp(timestamp); pt != nil {
		*t = *pt
	}
	return nil
}

func (t *Timestamp) String() string {
	if t == nil {
		return time.Time{}.UTC().Format(time.RFC3339Nano)
	}

	return t.UTC().Format(time.RFC3339Nano)
}
