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

package resources_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"

	"knative.dev/reconciler-test/pkg/manifest"
)

// TODO: this should be upstreamed to Reconciler-Test.

func ParseLocalYAML(images map[string]string, cfg map[string]interface{}) (map[string][]byte, error) {
	pwd, _ := os.Getwd()
	log.Println("PWD: ", pwd)
	_, filename, _, _ := runtime.Caller(1)

	yamls, err := manifest.ParseTemplates(path.Dir(filename), images, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates, %w", err)
	}
	_ = yamls
	log.Println("yamls: ", yamls)

	list, err := ioutil.ReadDir(yamls)
	if err != nil {
		return nil, fmt.Errorf("failed to read dir for parsed yaml, %w", err)
	}

	files := make(map[string][]byte, 1)

	for _, f := range list {
		name := path.Join(yamls, f.Name())
		if !f.IsDir() {
			read, err := ioutil.ReadFile(name)
			if err != nil {
				return nil, fmt.Errorf("failed to read file for parsed yaml, %w", err)
			}
			files[name] = read
		}
	}
	return files, nil
}

func RemoveBlanks(in string) string {
	in = strings.TrimSpace(in)
	// find one or more tabs and spaces ending with a new line.
	regex, err := regexp.Compile("[ |\t]+\n")
	if err != nil {
		return in
	}
	in = regex.ReplaceAllString(in, "")

	// find all two more more newlines and replaces them with a single.
	regex, err = regexp.Compile("\n{2,}")
	if err != nil {
		return in
	}
	return regex.ReplaceAllString(in, "\n")
}

func RemoveComments(in string) string {
	// find strings starting with # and ending with \n and remove them.
	regex, err := regexp.Compile("#.*\n")
	if err != nil {
		return in
	}
	return regex.ReplaceAllString(in, "")
}

func OutputYAML(files map[string][]byte) {
	more := false
	for _, file := range files {
		if more {
			fmt.Println("---")
		}
		more = true
		yaml := RemoveBlanks(RemoveComments(string(file)))
		fmt.Println(yaml)
	}
}
