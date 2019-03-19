package types

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
)

type URLRef struct {
	url.URL
}

func ParseURLRef(u string) *URLRef {
	if u == "" {
		return nil
	}
	pu, err := url.Parse(u)
	if err != nil {
		return nil
	}
	return &URLRef{URL: *pu}
}

// This allows json marshaling to always be in RFC3339Nano format.
func (u URLRef) MarshalJSON() ([]byte, error) {
	b := fmt.Sprintf("%q", u.String())
	return []byte(b), nil
}

func (u *URLRef) UnmarshalJSON(b []byte) error {
	var ref string
	if err := json.Unmarshal(b, &ref); err != nil {
		return err
	}
	r := ParseURLRef(ref)
	if r != nil {
		*u = *r
	}
	return nil
}

func (u URLRef) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	v := fmt.Sprintf("%s", u.String())
	return e.EncodeElement(v, start)
}

func (u *URLRef) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var ref string
	if err := d.DecodeElement(&ref, &start); err != nil {
		return err
	}
	r := ParseURLRef(ref)
	if r != nil {
		*u = *r
	}
	return nil
}

func (u *URLRef) String() string {
	if u == nil {
		return ""
	}
	return u.URL.String()
}
