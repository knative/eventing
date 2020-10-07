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
	"context"
	"errors"
	"io/ioutil"
	"os"
	"syscall"
	"testing"

	"github.com/wavesoftware/go-ensure"
	"knative.dev/eventing/test"

	"go.uber.org/zap"
	"knative.dev/eventing/test/upgrade/prober"
)

const (
	readyMessage = "prober ready"
)

var Log *zap.SugaredLogger

func TestContinuousEventsPropagationWithProber(t *testing.T) {
	// We run the prober as a golang test because it fits in nicely with
	// the rest of our integration tests, and AssertProberDefault needs
	// a *testing.T. Unfortunately, "go test" intercepts signals, so we
	// can't coordinate with the test by just sending e.g. SIGCONT, so we
	// create a named pipe and wait for the upgrade script to write to it
	// to signal that we should stop probing.
	ensureTempFilesAreCleaned()
	if err := syscall.Mkfifo(test.EventingFlags.PipeFile, 0666); err != nil {
		t.Fatal("Failed to create pipe:", err)
	}
	defer ensureTempFilesAreCleaned()
	client := setup(t, false)
	defer tearDown(client)

	config := prober.NewConfig(client.Namespace)

	// FIXME: https://github.com/knative/eventing/issues/2665
	config.FailOnErrors = false

	// Use zap.SugarLogger instead of t.Logf because we want to see failures
	// inline with other logs instead of buffered until the end.
	Log = createLogger()
	p := prober.NewProber(Log, client, config)
	p.Deploy(context.Background())
	ensure.NoError(ioutil.WriteFile(test.EventingFlags.ReadyFile, []byte(readyMessage), 0666))
	defer removeProbeWithEventsValidation(t, p, config.FailOnErrors)

	Log.Infof("Waiting for file: %v as a signal that "+
		"upgrade/downgrade is over, at which point we will finish the test "+
		"and check the prober.", test.EventingFlags.PipeFile)
	_, _ = ioutil.ReadFile(test.EventingFlags.PipeFile)
}

func removeProbeWithEventsValidation(t *testing.T, p prober.Prober, failOnErrors bool) {
	ctx := context.Background()
	p.Finish(ctx)
	assertEventProber(t, p, failOnErrors)
	p.Remove()
}

// assertEventProber verifies if all events are propagated correctly
func assertEventProber(t *testing.T, p prober.Prober, failOnErrors bool) {
	errs, events := parseReport(p.Verify())
	if len(errs) == 0 {
		t.Logf("All %d events propagated well", events)
	} else {
		t.Logf("There were %d events propagated, but %d errors occurred. "+
			"Listing them below.", events, len(errs))
	}
	reportErrors(t, errs, failOnErrors)
}

func reportErrors(t *testing.T, errors []error, failOnErrors bool) {
	for _, err := range errors {
		if failOnErrors {
			t.Error(err)
		} else {
			Log.Warn("Silenced FAIL: ", err)
		}
	}
	if len(errors) > 0 && !failOnErrors {
		t.Skipf(
			"Found %d errors, but FailOnErrors is false. Skipping test.",
			len(errors),
		)
	}
}

func parseReport(r prober.Report) ([]error, int) {
	errs := make([]error, 0)
	for _, t := range r.Thrown {
		errs = append(errs, errors.New(t))
	}
	return errs, r.Events
}

func createLogger() *zap.SugaredLogger {
	log, err := zap.NewDevelopment()
	ensure.NoError(err)
	return log.Sugar()
}

func ensureTempFilesAreCleaned() {
	filenames := []string{test.EventingFlags.PipeFile, test.EventingFlags.ReadyFile}
	for _, filename := range filenames {
		_, err := os.Stat(filename)
		if err == nil {
			ensure.NoError(os.Remove(filename))
		}
	}
}
