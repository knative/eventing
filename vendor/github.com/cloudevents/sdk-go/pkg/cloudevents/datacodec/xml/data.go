package xml

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strconv"
)

func Decode(in, out interface{}) error {
	if in == nil {
		return nil
	}

	b, ok := in.([]byte)
	if !ok {
		var err error
		b, err = xml.Marshal(in)
		if err != nil {
			return fmt.Errorf("[xml] failed to marshal in: %s", err.Error())
		}
	}

	// If the message is encoded as a base64 block as a string, we need to
	// decode that first before trying to unmarshal the bytes
	if len(b) > 1 && (b[0] == byte('"') || (b[0] == byte('\\') && b[1] == byte('"'))) {
		s, err := strconv.Unquote(string(b))
		if err != nil {
			return err
		}
		if len(s) > 0 && s[0] == '<' {
			// looks like xml, use it
			b = []byte(s)
		} else if len(s) > 0 {
			// looks like base64, decode
			bs, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return err
			}
			b = bs
		}
	}

	if err := xml.Unmarshal(b, out); err != nil {
		return fmt.Errorf("[xml] found bytes, but failed to unmarshal: %s %s", err.Error(), string(b))
	}
	return nil
}

func Encode(in interface{}) ([]byte, error) {
	if b, ok := in.([]byte); ok {
		// check to see if it is a pre-encoded byte string.
		if len(b) > 0 && b[0] == byte('"') {
			return b, nil
		}
	}

	return xml.Marshal(in)
}
