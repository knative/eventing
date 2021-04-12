package event

import (
	"gopkg.in/yaml.v2"
)

func ToYaml(event Event) ([]byte, error) {
	return yaml.Marshal(event)
}
