package main

import "knative.dev/eventing/test/prober/wathola/forwarder"

var instance forwarder.Forwarder

func main() {
	instance = forwarder.New()
	instance.Forward()
}
