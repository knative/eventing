package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"log"
	"net/http"
	"os"
)

const (
	// Environment variable containing the Slack App token.
	envSlackSecret = "SLACK_SECRET"

	// Environment variable containg a format string for the message to send to Slack.
	envMessageFormat = "MESSAGE_FORMAT"

	// Subtype for slack messages that are sent by Bots. See
	// https://api.slack.com/events/message/bot_message.
	botMessage = "bot_message"
)

type SlackResponderCredential struct {
	BotToken string
}

type slackResponder struct {
	api           *slack.Client
	messageFormat string
}

func (s *slackResponder) handleRequest(w http.ResponseWriter, r *http.Request) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	body := buf.String()
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), func(cfg *slackevents.Config) {
		// Trust that any message that came in is an event from Slack, do not verify using the
		// verification token.
		cfg.TokenVerified = true
	})
	if err != nil {
		log.Printf("Problem parsing the request: %+v :: %+v", err, r)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if eventsAPIEvent.Type != slackevents.CallbackEvent {
		log.Printf("Bad event type: %+v", eventsAPIEvent)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	innerEvent := eventsAPIEvent.InnerEvent
	switch ev := innerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		if ev.SubType == botMessage {
			// Don't respond to bots, or this will respond to itself, leading to an infinite loop.
			w.WriteHeader(http.StatusOK)
			return
		}
		msg := fmt.Sprintf(s.messageFormat, ev.Text)
		_, _, err := s.api.PostMessage(ev.Channel, msg, slack.PostMessageParameters{})
		if err != nil {
			log.Printf("Error posting message: %+v", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			log.Printf("Message posted.")
			w.WriteHeader(http.StatusOK)
		}
	default:
		log.Printf("Unknown event: %+v", innerEvent)
	}
}

func main() {
	flag.Parse()

	log.Printf("Starting slack responder on port 8080")

	slackSecret := os.Getenv(envSlackSecret)
	var slackCredential SlackResponderCredential
	err := json.Unmarshal([]byte(slackSecret), &slackCredential)
	if err != nil {
		log.Fatalf("Failed to unmarshal slack responder credentials: %s", err)
		return
	}

	messageFormat := os.Getenv(envMessageFormat)

	slackResponder := slackResponder{api: slack.New(slackCredential.BotToken), messageFormat: messageFormat}

	http.HandleFunc("/", slackResponder.handleRequest)
	http.ListenAndServe(":8080", nil)
}
