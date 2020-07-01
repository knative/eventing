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
	"os"
	"syscall"
	"testing"

	"knative.dev/eventing/test"

	"github.com/wavesoftware/go-ensure"
	"go.uber.org/zap"
	"knative.dev/eventing/test/upgrade/prober"
)

const (
	readyMessage = "prober ready"
)

func TestContinuousEventsPropagationWithProber(t *testing.T) {
	// We run the prober as a golang test because it fits in nicely with
	// the rest of our integration tests, and AssertProberDefault needs
	// a *testing.T. Unfortunately, "go test" intercepts signals, so we
	// can't coordinate with the test by just sending e.g. SIGCONT, so we
	// create a named pipe and wait for the upgrade script to write to it
	// to signal that we should stop probing.
	ensureTempFilesAreCleaned()
	if err := syscall.Mkfifo(test.EventingFlags.PipeFile, 0666); err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer ensureTempFilesAreCleaned()
	client := setup(t, false)
	defer tearDown(client)

	config := prober.NewConfig(client.Namespace)

	// FIXME: https://github.com/knative/eventing/issues/2665
	config.FailOnErrors = false

	// Use zap.SugarLogger instead of t.Logf because we want to see failures
	// inline with other logs instead of buffered until the end.
	log := createLogger()
	probe := prober.RunEventProber(log, client, config)
	ensure.NoError(ioutil.WriteFile(test.EventingFlags.ReadyFile, []byte(readyMessage), 0666))
	defer prober.AssertEventProber(t, probe)

	log.Infof("Waiting for file: %v as a signal that "+
		"upgrade/downgrade is over, at which point we will finish the test "+
		"and check the prober.", test.EventingFlags.PipeFile)
	_, _ = ioutil.ReadFile(test.EventingFlags.PipeFile)
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
