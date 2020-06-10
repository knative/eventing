package flags

import (
	"fmt"
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Channels holds the Channels we want to run test against.
type Channels []metav1.TypeMeta

func (channels *Channels) String() string {
	return fmt.Sprint(*channels)
}

// Set converts the input string to Channels.
func (channels *Channels) Set(value string) error {
	for _, channel := range strings.Split(value, ",") {
		channel := strings.TrimSpace(channel)
		split := strings.Split(channel, ":")
		if len(split) != 2 {
			log.Fatalf("The given Channel name %q is invalid, it needs to be in the form \"apiVersion:Kind\".", channel)
		}
		tm := metav1.TypeMeta{
			APIVersion: split[0],
			Kind:       split[1],
		}
		if !isValidChannel(tm.Kind) {
			log.Fatalf("The given channel name %q is invalid, tests cannot be run.\n", channel)
		}

		*channels = append(*channels, tm)
	}
	return nil
}

// Check if the channel name is valid.
func isValidChannel(channel string) bool {
	return strings.HasSuffix(channel, "Channel")
}
