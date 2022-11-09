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
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Parse parses YAML files into Unstructured objects.
//
// It supports 5 cases today:
//  1. pathname = path to a file --> parses that file.
//  2. pathname = path to a directory, recursive = false --> parses all files in
//     that directory.
//  3. pathname = path to a directory, recursive = true --> parses all files in
//     that directory and it's descendants
//  4. pathname = url --> fetches the contents of that URL and parses them as YAML.
//  5. pathname = combination of all previous cases, the string can contain
//     multiple records (file, directory or url) separated by comma
func Parse(pathname string, recursive bool) ([]unstructured.Unstructured, error) {

	pathnames := strings.Split(pathname, ",")
	aggregated := []unstructured.Unstructured{}
	for _, pth := range pathnames {
		els, err := read(pth, recursive)
		if err != nil {
			return nil, err
		}

		aggregated = append(aggregated, els...)
	}
	return aggregated, nil
}

// read contains logic to distinguish the type of record in pathname
// (file, directory or url) and calls the appropriate function
func read(pathname string, recursive bool) ([]unstructured.Unstructured, error) {
	info, err := os.Stat(pathname)
	if err != nil {
		if isURL(pathname) {
			return readURL(pathname)
		}
		return nil, err
	}

	if info.IsDir() {
		return readDir(pathname, recursive)
	}
	return readFile(pathname)
}

// readFile parses a single file.
func readFile(pathname string) ([]unstructured.Unstructured, error) {
	file, err := os.Open(pathname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return decode(file)
}

// readDir parses all files in a single directory and it's descendant directories
// if the recursive flag is set to true.
func readDir(pathname string, recursive bool) ([]unstructured.Unstructured, error) {
	list, err := ioutil.ReadDir(pathname)
	if err != nil {
		return nil, err
	}

	aggregated := []unstructured.Unstructured{}
	for _, f := range list {
		name := path.Join(pathname, f.Name())
		var els []unstructured.Unstructured

		switch {
		case f.IsDir() && recursive:
			els, err = readDir(name, recursive)
		case !f.IsDir():
			els, err = readFile(name)
		}

		if err != nil {
			return nil, err
		}
		aggregated = append(aggregated, els...)
	}
	return aggregated, nil
}

// readURL fetches a URL and parses its contents as YAML.
func readURL(url string) ([]unstructured.Unstructured, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return decode(resp.Body)
}

// decode consumes the given reader and parses its contents as YAML.
func decode(reader io.Reader) ([]unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLToJSONDecoder(reader)
	objs := []unstructured.Unstructured{}
	var err error
	for {
		out := unstructured.Unstructured{}
		err = decoder.Decode(&out)
		if err != nil {
			break
		}
		objs = append(objs, out)
	}
	if err != io.EOF {
		return nil, err
	}
	return objs, nil
}

// isURL checks whether or not the given path parses as a URL.
func isURL(pathname string) bool {
	uri, err := url.ParseRequestURI(pathname)
	if err != nil {
		return false
	}
	if uri.Scheme == "" {
		return false
	}
	return true
}
