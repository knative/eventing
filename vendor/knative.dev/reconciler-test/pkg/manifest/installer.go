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
	"bufio"
	"context"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
)

// CfgFn is the function signature of configuration mutation options.
type CfgFn func(map[string]interface{})

func InstallYaml(ctx context.Context, dir string, base map[string]interface{}) (Manifest, error) {
	env := environment.FromContext(ctx)
	cfg := env.TemplateConfig(base)
	f := feature.FromContext(ctx)

	yamls, err := ParseTemplates(dir, env.Images(), cfg)
	if err != nil {
		return nil, err
	}

	dynamicClient := dynamicclient.Get(ctx)

	manifest, err := NewYamlManifest(yamls, false, dynamicClient)
	if err != nil {
		return nil, err
	}

	// Apply yaml.
	if err := manifest.ApplyAll(); err != nil {
		return manifest, err
	}

	// Save the refs to Environment and Feature
	env.Reference(manifest.References()...)
	f.Reference(manifest.References()...)

	// Temp
	refs := manifest.References()
	log.Println("Created:")
	for _, ref := range refs {
		log.Println(ref)
	}
	return manifest, nil

}

func InstallLocalYaml(ctx context.Context, base map[string]interface{}) (Manifest, error) {
	pwd, _ := os.Getwd()
	log.Println("PWD: ", pwd)
	_, filename, _, _ := runtime.Caller(1)
	log.Println("FILENAME: ", filename)

	return InstallYaml(ctx, path.Dir(filename), base)
}

func ImagesLocalYaml() []string {
	pwd, _ := os.Getwd()
	log.Println("PWD: ", pwd)
	_, filename, _, _ := runtime.Caller(1)
	log.Println("FILENAME: ", filename)

	var images []string

	_ = filepath.Walk(path.Dir(filename), func(root string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), "yaml") {
			file, err := os.Open(root)
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				parts := strings.Split(scanner.Text(), "ko://")
				if len(parts) == 2 {
					image := strings.ReplaceAll(parts[1], "\"", "")
					images = append(images, image)
				}
			}
		}
		return nil
	})

	return images
}
