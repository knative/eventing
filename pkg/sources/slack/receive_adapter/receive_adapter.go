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
)

const (
	// Slack App token
	TOKEN = "1234asdf"
)

func main() {
	flag.Parse()

	log.Printf("Starting slack receive adapter web server on port 8080")

	eventHandler := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Inside slack receive adapter's event handler")
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		body := buf.String()
		eventsAPIEvent, e := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{TOKEN}))
		if e != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Handle the challenge event.
		if eventsAPIEvent.Type == slackevents.URLVerification {
			var r *slackevents.ChallengeResponse
			err := json.Unmarshal([]byte(body), &r)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text")
			w.Write([]byte(r.Challenge))
			return
		}

		if eventsAPIEvent.Type == slackevents.AppRateLimited {
			log.Printf("Rate limited: %v", eventsAPIEvent)
			return
		}

		// Handle all the 'normal' events.
		if eventsAPIEvent.Type == slackevents.CallbackEvent {
			cbEvent := eventsAPIEvent.Data.(*slackevents.EventsAPICallbackEvent)
			postMessage("foo", "bar", cbEvent)
		}
	}

	http.HandleFunc("/", eventHandler)
	http.ListenAndServe(":8080", nil)
	//eh := http.HandlerFunc(eventHandler)
	//http.ListenAndServe(":8080", eh)
}

func postMessage(target, source string, m *slackevents.EventsAPICallbackEvent) error {
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
