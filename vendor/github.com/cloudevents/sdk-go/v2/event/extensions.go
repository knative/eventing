package event

import (
	"strings"
)

const (
	// DataContentEncodingKey is the key to DeprecatedDataContentEncoding for versions that do not support data content encoding
	// directly.
	DataContentEncodingKey = "datacontentencoding"
)

func caseInsensitiveSearch(key string, space map[string]interface{}) (interface{}, bool) {
	lkey := strings.ToLower(key)
	for k, v := range space {
		if strings.EqualFold(lkey, strings.ToLower(k)) {
			return v, true
		}
	}
	return nil, false
}

func IsExtensionNameValid(key string) bool {
	if len(key) < 1 || len(key) > 20 {
		return false
	}
	for _, c := range key {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
