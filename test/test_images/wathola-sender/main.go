package main

import "knative.dev/eventing/test/prober/wathola/sender"

func main() {
	sender.New().SendContinually()
}
