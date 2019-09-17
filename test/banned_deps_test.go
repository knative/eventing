/*
Copyright 2019 The Knative Authors

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

package test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type vbTuple struct {
	banned string
	vanity string
}

var bannedImports = []vbTuple{
	{
		banned: "github.com/knative",
		vanity: "knative.dev",
	}, {
		banned: "github.com/kubernetes",
		vanity: "k8s.io",
	}, {
		banned: "github.com/istio",
		vanity: "istio.io",
	},
}

func TestBannedImports(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	dir = fmt.Sprintf("%s%s..%s%s", dir, string(filepath.Separator), string(filepath.Separator), "vendor")

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			return nil
		}
		for _, vb := range bannedImports {
			if strings.HasSuffix(path, vb.banned) {
				return fmt.Errorf("%q is a banned import, use vanity import instead %q", vb.banned, vb.vanity)
			}
		}
		return nil
	})
	if err != nil {
		t.Errorf(err.Error())
	}
}
