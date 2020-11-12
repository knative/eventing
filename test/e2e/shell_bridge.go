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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/magefile/mage/sh"
)

func ShellOutFunction(funcName string) error {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return fmt.Errorf("can't get caller for self")
	}
	e2eCommonScriptPath := path.Join(path.Dir(filename), "../e2e-common.sh")
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -Eeuo pipefail
source %s

%s
`, e2eCommonScriptPath, funcName)
	tmpfile, err := ioutil.TempFile("", funcName+"-*.sh")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(tmpfile.Name(), []byte(script), 0755)
	if err != nil {
		return err
	}

	defer func() {
		// clean up
		_ = os.Remove(tmpfile.Name())
	}()
	return sh.RunWithV(environment(os.Environ(), keyval), tmpfile.Name())
}

func environment(data []string, keyval func(item string) (key, val string)) map[string]string {
	items := make(map[string]string)
	for _, item := range data {
		key, val := keyval(item)
		items[key] = val
	}
	return items
}

func keyval(item string) (key, val string) {
	splits := strings.Split(item, "=")
	key = splits[0]
	val = splits[1]
	return
}
