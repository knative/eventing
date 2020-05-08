package main

import (
	"testing"
	"time"

	"github.com/phayes/freeport"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/prober/wathola/config"
	"knative.dev/eventing/test/prober/wathola/receiver"

	"github.com/stretchr/testify/assert"
)

func TestReceiverMain(t *testing.T) {
	config.Instance.Receiver.Port = freeport.GetPort()
	go main()
	defer receiver.Stop()
	err := lib.WaitUntil(receiver.IsRunning, 10*time.Minute)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, instance)
}
