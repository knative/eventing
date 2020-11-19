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

package manifest

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ParseTemplates walks through all the template yaml file in the given directory
// and produces instantiated yaml file in a temporary directory.
// Return the name of the temporary directory
func ParseTemplates(path string, images map[string]string, data map[string]interface{}) (string, error) {
	dir, err := ioutil.TempDir("", "processed_yaml")
	if err != nil {
		panic(err)
	}

	err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), "yaml") {
			t, err := template.ParseFiles(path)
			if err != nil {
				return err
			}
			tmpfile, err := ioutil.TempFile(dir, strings.Replace(info.Name(), ".yaml", "-*.yaml", 1))
			if err != nil {
				log.Fatal(err)
			}
			err = t.Execute(tmpfile, data)
			if err != nil {
				log.Print("execute: ", err)
				return err
			}
			_ = tmpfile.Close()

			// Set image.
			read, err := ioutil.ReadFile(tmpfile.Name())
			newContents := string(read)
			for key, image := range images {
				newContents = strings.Replace(newContents, key, image, -1)
			}

			err = ioutil.WriteFile(tmpfile.Name(), []byte(newContents), 0)
			if err != nil {
				log.Print("write file: ", err)
				return err
			}
		}
		return nil
	})
	log.Print("new files in ", dir)
	if err != nil {
		return "", err
	}
	return dir, nil
}

// ExecuteTemplate instantiates the given template with data
func ExecuteTemplate(tpl string, data map[string]interface{}) (string, error) {
	t, err := template.New("").Parse(tpl)
	if err != nil {
		panic(err)
	}
	buffer := &bytes.Buffer{}
	err = t.Execute(buffer, data)
	return buffer.String(), err
}
