/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestShellScript(t *testing.T) {
	tests := []struct {
		label   string
		script  string
		wantErr bool
		wants   []string
	}{
		{
			"echo", "echo unit test", false,
			[]string{"echo [OUT] unit test\n"},
		}, {
			"echo-err", "echo unit test 1>&2", false,
			[]string{"echo-err [ERR] unit test\n"},
		}, {
			"exit", "exit 45", true,
			[]string{},
		},
	}
	assert := assertions{t: t}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			out := captureOutput(func() {
				if err := ShellScript(tt.label, tt.script); (err != nil) != tt.wantErr {
					t.Errorf("ShellScript() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
			assert.textContains(out, tt.wants)
		})
	}
}

func captureOutput(f func()) string {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	stdout := os.Stdout
	stderr := os.Stderr
	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
		log.SetOutput(os.Stderr)
	}()
	os.Stdout = writer
	os.Stderr = writer
	log.SetOutput(writer)
	out := make(chan string)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		var buf bytes.Buffer
		wg.Done()
		_, _ = io.Copy(&buf, reader)
		out <- buf.String()
	}()
	wg.Wait()
	f()
	_ = writer.Close()
	return <-out
}

type assertions struct {
	t *testing.T
}

func (a assertions) textContains(haystack string, needles []string) {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			a.t.Errorf(
				"expected %q is not in: %q",
				needle, haystack,
			)
		}
	}
}
