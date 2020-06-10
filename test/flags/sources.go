package flags

import (
	"fmt"
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Sources holds the Sourcess we want to run test against.
type Sources []metav1.TypeMeta

func (sources *Sources) String() string {
	return fmt.Sprint(*sources)
}

// Set converts the input string to Sources.
func (sources *Sources) Set(value string) error {
	for _, source := range strings.Split(value, ",") {
		source := strings.TrimSpace(source)
		split := strings.Split(source, ":")
		if len(split) != 2 {
			log.Fatalf("The given Source name %q is invalid, it needs to be in the form \"apiVersion:Kind\".", source)
		}
		tm := metav1.TypeMeta{
			APIVersion: split[0],
			Kind:       split[1],
		}
		if !isValidSource(tm.Kind) {
			log.Fatalf("The given source name %q is invalid, tests cannot be run.\n", source)
		}

		*sources = append(*sources, tm)
	}
	return nil
}

// Check if the Source name is valid.
func isValidSource(source string) bool {
	return strings.HasSuffix(source, "Source")
}
