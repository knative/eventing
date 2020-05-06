package main

import "knative.dev/eventing/test/prober/wathola/receiver"

var instance receiver.Receiver

func main() {
	instance = receiver.New()
	instance.Receive()
}
