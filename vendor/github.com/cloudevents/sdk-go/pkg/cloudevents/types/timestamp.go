package types

import (
	"encoding/json"
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
	rfc3339 := fmt.Sprintf("%q", t.Format(time.RFC3339Nano))
	return []byte(rfc3339), nil
}

func (t *Timestamp) UnmarshalJSON(b []byte) error {
	var timestamp string
	if err := json.Unmarshal(b, &timestamp); err != nil {
		return err
	}
	pt := ParseTimestamp(timestamp)
	if pt != nil {
		*t = *pt
	}
	return nil
}

func (t *Timestamp) String() string {
	if t == nil {
		return time.Time{}.Format(time.RFC3339Nano)
	}

	return t.Format(time.RFC3339Nano)
}
