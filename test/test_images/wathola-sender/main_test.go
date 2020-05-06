package main

import (
	"syscall"
	"testing"
	"time"

	"github.com/wavesoftware/go-ensure"
)

func TestSenderMain(t *testing.T) {
	p := syscall.Getpid()
	go main()
	time.Sleep(time.Second)
	err := syscall.Kill(p, syscall.SIGINT)
	ensure.NoError(err)
}
