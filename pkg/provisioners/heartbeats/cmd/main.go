package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type heartbeat struct {
	Sequence int    `json:"id"`
	Label    string `json:"label"`
}

var (
	remote   string
	label    string
	period   int
	sequence int
	hb       *heartbeat
)

func init() {
	flag.StringVar(&remote, "remote", "", "the host url to heartbeat to")
	flag.StringVar(&label, "label", "", "a special label")
	flag.IntVar(&period, "period", 5, "the number of seconds between heartbeats")
}

func main() {
	flag.Parse()

	hb = &heartbeat{
		Sequence: 0,
		Label:    label,
	}

	for {
		send()
		time.Sleep(time.Duration(period) * time.Second)
	}
}

func send() {
	sequence++
	resp, err := http.Post(remote, "application/json", body())
	if err != nil {
		log.Printf("Unable to make request: %v, %+v", sequence, err)
		return
	}
	defer resp.Body.Close()
}

func body() io.Reader {
	b, err := json.Marshal(hb)
	if err == nil {
		return strings.NewReader("{\"error\":\"true\"}")
	}
	return bytes.NewBuffer(b)
}
