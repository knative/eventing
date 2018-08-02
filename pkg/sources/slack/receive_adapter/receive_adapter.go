package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"github.com/nlopes/slack/slackevents"
	"log"
	"net/http"
	"time"
	"github.com/knative/eventing/pkg/event"
	"fmt"
	"io/ioutil"
	"os"
	"github.com/golang/glog"
)

const (
	// Target for messages.
	envTarget = "TARGET"

	// Environment variable containing the Slack App token.
	envSlackSecret = "SLACK_SECRET"
)

type SlackCredential struct {
	Token string
}

func main() {
	flag.Parse()

	log.Printf("Starting slack receive adapter web server on port 8080")

	target := os.Getenv(envTarget)

	slackSecret := os.Getenv(envSlackSecret)
	var slackCredential SlackCredential
	err := json.Unmarshal([]byte(slackSecret), &slackCredential)
	if err != nil {
		glog.Fatalf("Failed to unmarshal slack credentials: %s", err)
		return
	}

	eventHandler := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Inside slack receive adapter's event handler")

		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		body := buf.String()
		eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{slackCredential.Token}))
		if err != nil {
			log.Printf("Problem parsing the event: %+v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		switch eventsAPIEvent.Type {
		case slackevents.URLVerification:
			// Handle the challenge event.
			var r *slackevents.ChallengeResponse
			err := json.Unmarshal([]byte(body), &r)
			if err != nil {
				log.Printf("Problem unmarshaling the URLVerification event: %+v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text")
			w.Write([]byte(r.Challenge))
			log.Printf("Responded to URLVerification event")

		case slackevents.AppRateLimited:
			log.Printf("Rate limited: %v", eventsAPIEvent)

		case slackevents.CallbackEvent:
			// Handle all the 'normal' events.
			cbEvent := eventsAPIEvent.Data.(*slackevents.EventsAPICallbackEvent)
			postMessage(target, cbEvent)

		default:
			log.Printf("Unknown slack event type: %v", eventsAPIEvent.Type)
		}
	}

	http.HandleFunc("/", eventHandler)
	http.ListenAndServe(":8080", nil)
	//eh := http.HandlerFunc(eventHandler)
	//http.ListenAndServe(":8080", eh)
}

func postMessage(target string, m *slackevents.EventsAPICallbackEvent) error {
	//log.Printf("postMessage: %v, %v, %v", target, source, m)
	//return nil
	ctx := event.EventContext{
		CloudEventsVersion: event.CloudEventsVersion,
		EventType:          "slack.foobar.event_type",
		EventID:            m.EventID,
		EventTime:          parseTime(m.EventTime),
		Source:             "slack.foobar.hardcoded_source",
	}
	URL := fmt.Sprintf("http://%s/", target)
	req, err := event.Binary.NewRequest(URL, m, ctx)
	if err != nil {
		log.Printf("Failed to marshal the message: %+v : %s", m, err)
		return err
	}

	log.Printf("Posting to %q", URL)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	log.Printf("response Status: %s", resp.Status)
	body, _ := ioutil.ReadAll(resp.Body)
	log.Printf("response Body: %s", string(body))
	return nil
}

func parseTime(unixTime int) time.Time {
	return time.Unix(int64(unixTime), 0)
}
