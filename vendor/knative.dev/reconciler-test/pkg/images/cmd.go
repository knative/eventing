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

package images

import (
	"os"
	"os/exec"
	"strings"
)

// Helper functions to run shell commands.

func cmd(cmdLine string) *exec.Cmd {
	cmdSplit := strings.Split(cmdLine, " ")
	cmd := cmdSplit[0]
	args := cmdSplit[1:]
	c := exec.Command(cmd, args...)
	c.Stderr = os.Stdout // Pipe the stderr in stdout
	return c
}

func runCmd(cmdLine string) (string, error) {
	cmd := cmd(cmdLine)

	cmdOut, err := cmd.Output()
	return string(cmdOut), err
}
