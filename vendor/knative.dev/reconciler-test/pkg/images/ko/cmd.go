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

package ko

import (
	"bufio"
	"context"
	"io/ioutil"
	"os/exec"

	"knative.dev/pkg/logging"
)

// Helper functions to run shell commands.

func cmd(ctx context.Context, cmds []string) ([]byte, error) {
	log := logging.FromContext(ctx)
	prog := cmds[0]
	args := cmds[1:]
	c := exec.Command(prog, args...)

	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}

	// start the command after having set up the pipe
	if err = c.Start(); err != nil {
		return nil, err
	}

	// read command's stderr line by line
	in := bufio.NewScanner(stderr)

	for in.Scan() {
		msg := in.Text()
		log.Debug(msg) // write each line to your log, or anything you need
	}
	if err = in.Err(); err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, err
	}

	return bytes, c.Wait()
}

func runCmd(ctx context.Context, cmds []string) (string, error) {
	logging.FromContext(ctx).Debugf("google/ko publish: %q", cmds)
	stdout, err := cmd(ctx, cmds)
	if err != nil {
		return "", err
	}

	return string(stdout), err
}
