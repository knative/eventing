package sources

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

func RunEventSource(es EventSource) {
	op := os.Getenv(FeedOperationKey)
	feedContext := os.Getenv(FeedContextKey)
	trigger := os.Getenv(FeedTriggerKey)
	target := os.Getenv(FeedTargetKey)

	decodedContext, _ := base64.StdEncoding.DecodeString(feedContext)
	decodedTrigger, _ := base64.StdEncoding.DecodeString(trigger)
	var c FeedContext
	var t EventTrigger

	err := json.Unmarshal(decodedContext, &c)
	if err != nil {
		panic(fmt.Sprintf("can not unmarshal %q %q : %s", FeedContextKey, decodedContext, err))
	}

	err = json.Unmarshal(decodedTrigger, &t)
	if err != nil {
		panic(fmt.Sprintf("can not unmarshal %q %q : %s", FeedTriggerKey, decodedTrigger, err))
	}

	log.Printf("Doing %q target: %q Received feedContext as %+v trigger %+v", op, target, c, t)
	if op == string(StartFeed) {
		b, err := es.StartFeed(t, target)
		if err != nil {
			panic(fmt.Sprintf("Failed to start feed: %s", err))
		}

		marshalledFeedContext, err := json.Marshal(b)
		if err != nil {
			panic(fmt.Sprintf("Failed to marshal returned feed context %+v : %s", b, err))
		}
		encodedFeedContext := base64.StdEncoding.EncodeToString(marshalledFeedContext)

		err = ioutil.WriteFile("/dev/termination-log", []byte(encodedFeedContext), os.ModeDevice)
		if err != nil {
			panic(fmt.Sprintf("Failed to write the termination message: %s", err))
		}
	} else {
		err = es.StopFeed(t, c)
		if err != nil {
			panic(fmt.Sprintf("Failed to stop feed: %s", err))
		}
	}
}
