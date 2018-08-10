package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/knative/eventing/pkg/event"
	"github.com/nlopes/slack/slackevents"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	// Target for messages.
	envTarget = "TARGET"

	// Environment variable containing the Slack App token.
	envSlackSecret = "SLACK_SECRET"
)

type slackHandler struct {
	target string
	cred   *SlackCredential
}

type SlackCredential struct {
	Token string
}

func newSlackHandler(target string, cred *SlackCredential) *slackHandler {
	return &slackHandler{
		target: target,
		cred:   cred,
	}
}

func main() {
	flag.Parse()

	target := os.Getenv(envTarget)

	log.Printf("Starting slack receive adapter web server on port 8080. Target: %q", target)

	slackSecret := os.Getenv(envSlackSecret)
	var slackCredential SlackCredential
	err := json.Unmarshal([]byte(slackSecret), &slackCredential)
	if err != nil {
		log.Fatalf("Failed to unmarshal slack credentials: %s", err)
		return
	}

	h := newSlackHandler(target, &slackCredential)

	http.HandleFunc("/", h.handleEvent)
	http.ListenAndServe(":8080", nil)
}

func (h *slackHandler) handleEvent(w http.ResponseWriter, r *http.Request) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	body := buf.String()
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: h.cred.Token}))
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
		h.postMessage(cbEvent)

	default:
		log.Printf("Unknown slack event type: %v", eventsAPIEvent.Type)
	}
}

func (h *slackHandler) postMessage(m *slackevents.EventsAPICallbackEvent) error {
	ctx := event.EventContext{
		CloudEventsVersion: event.CloudEventsVersion,
		EventType:          "dev.knative.slack.event",
		EventID:            m.EventID,
		EventTime:          parseTime(m.EventTime),
		Source:             m.APIAppID,
	}
	URL := fmt.Sprintf("http://%s/", h.target)
	req, err := event.Binary.NewRequest(URL, m, ctx)
	if err != nil {
		log.Printf("Failed to marshal the message: %+v : %s", m, err)
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to post the message: %v, %v", URL, err)
		return err
	}
	defer resp.Body.Close()
	log.Printf("response Status: %s", resp.Status)
	return nil
}

func parseTime(unixTime int) time.Time {
	return time.Unix(int64(unixTime), 0)
}
