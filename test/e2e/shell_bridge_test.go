/*
 * Copyright 2020 The Knative Authors
 *
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

package e2e

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"testing"
	"time"
)

func TestShellScript(t *testing.T) {
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		label   string
		script  string
		wantErr bool
		out     string
	}{
		{
			"shell", "LANG=C header utest", false,
			fmt.Sprintf(`shell [OUT] ===============
shell [OUT] ==== UTEST ====
shell [OUT] ===============
shell [OUT] ==== %s
shell [OUT] ===============
`, time.Now().In(la).Format(time.UnixDate)),
		},
		{
			"sh", "exit 45", true,
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.script, func(t *testing.T) {
			out := captureOutput(func() {
				if err := ShellScript(tt.label, tt.script); (err != nil) != tt.wantErr {
					t.Errorf("ShellScript() error = %v, wantErr %v", err, tt.wantErr)
				}
			})

			if out != tt.out {
				t.Errorf("actual = %v\n  want = %v\n", out, tt.out)
			}
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