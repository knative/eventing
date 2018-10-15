package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Heartbeat struct {
	Sequence int    `json:"id"`
	Label    string `json:"label"`
}

var (
	remote    string
	label     string
	periodStr string
	period    int
	sequence  int
	hb        *Heartbeat
)

func init() {
	flag.StringVar(&remote, "remote", "", "the host url to heartbeat to")
	flag.StringVar(&label, "label", "", "a special label")
	flag.StringVar(&periodStr, "period", "5", "the number of seconds between heartbeats")
}

func main() {
	flag.Parse()

	hb = &Heartbeat{
		Sequence: 0,
		Label:    label,
	}

	period, err := strconv.Atoi(periodStr)
	if err != nil {
		period = 5
	}

	for {
		send()
		time.Sleep(time.Duration(period) * time.Second)
	}
}

func send() {
	hb.Sequence++
	resp, err := http.Post(remote, "application/json", body())
	if err != nil {
		log.Printf("Unable to make request: %+v, %v", hb, err)
		return
	} else {
		log.Printf("[%d]: %+v", resp.StatusCode, hb)
	}
	defer resp.Body.Close()
}

func body() io.Reader {
	b, err := json.Marshal(hb)
	if err != nil {
		return strings.NewReader("{\"error\":\"true\"}")
	}
	return bytes.NewBuffer(b)
}
