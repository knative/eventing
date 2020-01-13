// +build probe

/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package upgrade

import (
	"io/ioutil"
	"knative.dev/eventing/test/common"
	"knative.dev/eventing/test/prober"
	"log"
	"os"
	"syscall"
	"testing"
	"time"
)

const (
	pipe         = "/tmp/prober-signal"
	ready        = "/tmp/prober-ready"
	readyMessage = "prober ready"
)

var (
	// FIXME: Interval is set to 200 msec, as lower values will result in errors
	// https://github.com/knative/eventing/issues/2357
	fixmeInterval = 200 * time.Millisecond
)

func TestProbe(t *testing.T) {
	// We run the prober as a golang test because it fits in nicely with
	// the rest of our integration tests, and AssertProberDefault needs
	// a *testing.T. Unfortunately, "go test" intercepts signals, so we
	// can't coordinate with the test by just sending e.g. SIGCONT, so we
	// create a named pipe and wait for the upgrade script to write to it
	// to signal that we should stop probing.
	if err := syscall.Mkfifo(pipe, 0666); err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer cleanupTempFiles()
	client := setup(t, false)
	defer tearDown(client)

	// Use log.Printf instead of t.Logf because we want to see failures
	// inline with other logs instead of buffered until the end.
	config := prober.NewConfig(client.Namespace)
	config.Interval = fixmeInterval
	probe := prober.RunEventProber(log.Printf, client, config)
	common.NoError(ioutil.WriteFile(ready, []byte(readyMessage), 0666))
	defer prober.AssertEventProber(t, probe)

	// e2e-upgrade-test.sh will close this pipe to signal the upgrade is
	// over, at which point we will finish the test and check the prober.
	_, _ = ioutil.ReadFile(pipe)
}

func cleanupTempFiles() {
	filenames := []string{pipe, ready}
	for _, filename := range filenames {
		_, err := os.Stat(filename)
		if err == nil {
			common.NoError(os.Remove(filename))
		}
	}
}