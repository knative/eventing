package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/elafros/eventing/pkg/event"
	"github.com/golang/glog"
)

const (
	databaseURL = "https://inlined-junkdrawer.firebaseio.com/"
	address     = ":8080"
)

// SendEventToDB...
func SendEventToDB(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	context, err := event.FromRequest(&data, r)
	if err != nil {
		glog.Errorf("Failed to parse event: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	url, err := url.Parse(databaseURL + "seenEvents/" + context.EventID)
	if err != nil {
		glog.Errorf("Failed to parse url %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Can not fail
	body, _ := json.Marshal(data)

	write := &http.Request{
		Method: http.MethodPut,
		URL:    url,
		Body:   ioutil.NopCloser(bytes.NewReader(body)),
	}
	if _, err := http.DefaultClient.Do(write); err != nil {
		glog.Errorf("Failed to write to RTDB: %s", err)
	}
	w.WriteHeader(http.StatusOK)
}

func main() {
	http.ListenAndServe(address, http.HandlerFunc(SendEventToDB))
}
