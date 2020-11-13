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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"runtime"
)

// ShellScript calls a shell function defined in e2e-common shell library.
func ShellScript(label, script string) error {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("can't get caller for self")
	}
	rootPath := path.Join(path.Dir(filename), "../..")
	scriptContent := fmt.Sprintf(`#!/usr/bin/env bash

set -Eeuo pipefail

cd "%s"
source ./test/e2e-common.sh

%s
`, rootPath, script)
	tmpfile, err := ioutil.TempFile("", "shellout-*.sh")
	if err != nil {
		return err
	}
	_, err = tmpfile.WriteString(scriptContent)
	if err != nil {
		return err
	}
	err = tmpfile.Chmod(0700)
	if err != nil {
		return err
	}
	err = tmpfile.Close()
	if err != nil {
		return err
	}

	defer func() {
		// clean up
		_ = os.Remove(tmpfile.Name())
	}()
	c := exec.Command(tmpfile.Name())
	c.Env = os.Environ()
	c.Stdout = NewPrefixer(os.Stdout, label + " [OUT] ")
	c.Stderr = NewPrefixer(os.Stderr, label + " [ERR] ")
	return c.Run()
}

type prefixer struct {
	prefix      		string
	writer          io.Writer
	trailingNewline bool
	buf             bytes.Buffer // reuse buffer to save allocations
}

// NewPrefixer creates a new prefixer that forwards all calls to Write() to
// writer.Write() with all lines prefixed with the value of prefix. Having a
// function instead of a static prefix allows to print timestamps or other
// changing information.
func NewPrefixer(writer io.Writer, prefix string) io.Writer {
	return &prefixer{prefix: prefix, writer: writer, trailingNewline: true}
}

func (pf *prefixer) Write(payload []byte) (int, error) {
	pf.buf.Reset() // clear the buffer

	for _, b := range payload {
		if pf.trailingNewline {
			pf.buf.WriteString(pf.prefix)
			pf.trailingNewline = false
		}

		pf.buf.WriteByte(b)

		if b == '\n' {
			// do not print the prefix right after the newline character as this might
			// be the very last character of the stream and we want to avoid a trailing prefix.
			pf.trailingNewline = true
		}
	}

	n, err := pf.writer.Write(pf.buf.Bytes())
	if err != nil {
		// never return more than original length to satisfy io.Writer interface
		if n > len(payload) {
			n = len(payload)
		}
		return n, err
	}

	// return original length to satisfy io.Writer interface
	return len(payload), nil
}

// EnsureNewline prints a newline if the last character written wasn't a newline unless nothing has ever been written.
// The purpose of this method is to avoid ending the output in the middle of the line.
func (pf *prefixer) EnsureNewline() {
	if !pf.trailingNewline {
		_, _ = fmt.Fprintln(pf.writer)
	}
}
