package main

import (
	"github.com/phayes/freeport"
	"knative.dev/eventing/test/prober/wathola/config"
	"knative.dev/eventing/test/prober/wathola/receiver"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReceiverMain(t *testing.T) {
	config.Instance.Receiver.Port = freeport.GetPort()
	go main()
	defer receiver.Stop()

	time.Sleep(time.Second)

	assert.NotNil(t, instance)
}
