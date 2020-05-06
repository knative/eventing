package main

import (
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"knative.dev/eventing/test/prober/wathola/config"
	"knative.dev/eventing/test/prober/wathola/forwarder"

	"testing"
	"time"
)

func TestForwarderMain(t *testing.T) {
	config.Instance.Forwarder.Port = freeport.GetPort()
	go main()
	defer forwarder.Stop()

	time.Sleep(time.Second)

	assert.NotNil(t, instance)
}
