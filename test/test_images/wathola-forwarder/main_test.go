package main

import (
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/prober/wathola/config"
	"knative.dev/eventing/test/prober/wathola/forwarder"
)

func TestForwarderMain(t *testing.T) {
	config.Instance.Forwarder.Port = freeport.GetPort()
	go main()
	defer forwarder.Stop()
	err := lib.WaitUntil(forwarder.IsRunning, 10*time.Minute)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, instance)
}
