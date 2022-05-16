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
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

// ExecuteTemplates executes a set of templates found at path, filtering on
// suffix. Executed into memory and returned.
func ExecuteTemplates(path, suffix string, images map[string]string, data map[string]interface{}) (map[string]string, error) {
	files := make(map[string]string, 1)

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), suffix) {
			t, err := template.ParseFiles(path)
			if err != nil {
				log.Print("parse: ", err)
				return err
			}
			buffer := &bytes.Buffer{}

			// Execute the template and save the result to the buffer.
			err = t.Execute(buffer, data)
			if err != nil {
				log.Print("execute: ", err)
				return err
			}

			// Set image.
			yaml := buffer.String()
			for key, image := range images {
				yaml = strings.Replace(yaml, key, image, -1)
			}

			files[path] = yaml
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return files, nil
}

// executeTemplatesFS executes a set of templates in the given filesystem, filtering on
// suffix. Executed into memory and returned.
func executeTemplatesFS(fsys fs.FS, suffix string, images map[string]string, data map[string]interface{}) (map[string]string, error) {
	files := make(map[string]string, 1)

	err := fs.WalkDir(fsys, ".", func(path string, info fs.DirEntry, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), suffix) {
			t, err := template.ParseFS(fsys, path)
			if err != nil {
				log.Print("parse: ", err)
				return err
			}
			buffer := &bytes.Buffer{}

			// Execute the template and save the result to the buffer.
			err = t.Execute(buffer, data)
			if err != nil {
				log.Print("execute: ", err)
				return err
			}

			// Set image.
			yaml := buffer.String()
			for key, image := range images {
				yaml = strings.Replace(yaml, key, image, -1)
			}

			files[path] = yaml
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return files, nil
}

// ParseTemplates walks through all the template yaml file in the given directory
// and produces instantiated yaml file in a temporary directory.
// Returns the name of the temporary directory.
// Deprecated: Use ParseTemplatesFS.
func ParseTemplates(path string, images map[string]string, cfg map[string]interface{}) (string, error) {
	return templatesToTmp(ExecuteTemplates(path, "yaml", images, cfg))
}

// ParseTemplatesFS walks through all the template yaml file in the given FS
// and produces instantiated yaml file in a temporary directory.
// Returns the name of the temporary directory.
func ParseTemplatesFS(fsys fs.FS, images map[string]string, cfg map[string]interface{}) (string, error) {
	return templatesToTmp(executeTemplatesFS(fsys, "yaml", images, cfg))
}

func templatesToTmp(files map[string]string, err error) (string, error) {
	if err != nil {
		return "", err
	}

	tmpDir, err := ioutil.TempDir("", "processed_yaml")
	if err != nil {
		panic(err)
	}

	for file, contents := range files {
		name := filepath.Base(filepath.Base(file))
		name = strings.Replace(name, ".yaml", "-*.yaml", 1)

		tmpFile, err := ioutil.TempFile(tmpDir, name)
		if err != nil {
			panic(err)
		}
		_, _ = tmpFile.WriteString(contents)
	}

	log.Print("new files in ", tmpDir)
	return tmpDir, nil
}

// ExecuteYAML process the templates found in files named "*.yaml" and return the f.
func ExecuteYAML(fsys fs.FS, images map[string]string, cfg map[string]interface{}) (map[string]string, error) {
	return executeTemplatesFS(fsys, "yaml", images, cfg)
}

func removeBlanks(in string) string {
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

func removeComments(in string) string {
	// find strings starting with # and ending with \n and remove them.
	regex, err := regexp.Compile("#.*\n")
	if err != nil {
		return in
	}
	return regex.ReplaceAllString(in, "")
}

// OutputYAML writes out each file contents  to out after removing comments and
// blank lines. This also adds a YAML file separator "---" between each file.
// Files is a map of "filename" to "file contents".
func OutputYAML(out io.Writer, files map[string]string) {
	names := make([]string, 0)
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	more := false
	for _, name := range names {
		file := files[name]

		if more {
			_, _ = out.Write([]byte("---\n"))
		}
		more = true
		yaml := removeBlanks(removeComments(file))
		_, _ = out.Write([]byte(yaml))
		_, _ = out.Write([]byte("\n"))
	}
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
