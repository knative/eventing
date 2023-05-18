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
	"encoding/json"
	"io/fs"
	"strings"

	"k8s.io/client-go/util/retry"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
)

// CfgFn is the function signature of configuration mutation options.
type CfgFn func(map[string]interface{})

func InstallYamlFS(ctx context.Context, fsys fs.FS, base map[string]interface{}) (Manifest, error) {
	env := environment.FromContext(ctx)
	images, err := environment.ProduceImages(ctx)
	if err != nil {
		return nil, err
	}
	cfg := env.TemplateConfig(base)
	f := feature.FromContext(ctx)
	log := loggingFrom(ctx, "InstallYamlFS")

	yamlsDir, err := ParseTemplatesFS(ctx, fsys, images, cfg)
	if err != nil {
		return nil, err
	}

	dynamicClient := dynamicclient.Get(ctx)

	manifest, err := NewYamlManifest(ctx, yamlsDir, false, dynamicClient)
	if err != nil {
		return nil, err
	}

	// Apply yaml.
	err = retry.OnError(retry.DefaultRetry, isWebhookError, func() error {
		// This is a workaround for https://github.com/knative/pkg/issues/1509
		// Because tests currently fail immediately on any creation failure, this
		// is problematic. On the reconcilers it's not an issue because they recover,
		// but tests need this retry.
		return manifest.ApplyAll()
	})
	if err != nil {
		return manifest, err
	}

	// Save the refs to Environment and Feature
	env.Reference(manifest.References()...)
	if f != nil {
		f.Reference(manifest.References()...)
	}

	// Temp
	refs := manifest.References()
	if j, err := json.MarshalIndent(refs, "", "  "); err == nil {
		log.Debug("Created: ", string(j))
	} else {
		log.Fatal(err)
	}

	return manifest, nil
}

func ImagesFromFS(ctx context.Context, fsys fs.FS) []string {
	log := logging.FromContext(ctx)
	var images []string

	_ = fs.WalkDir(fsys, ".", func(path string, info fs.DirEntry, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), "yaml") {
			file, err := fsys.Open(path)
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

func isWebhookError(err error) bool {
	str := err.Error()

	// isEOFError is a workaround for https://github.com/knative/pkg/issues/1509.
	// Example error:
	// Internal error occurred: failed calling webhook "defaulting.webhook.kafka.eventing.knative.dev": Post "https://kafka-webhook-eventing.knative-eventing.svc:443/defaulting?timeout=2s": EOF
	isEOFError := strings.Contains(str, "webhook") &&
		strings.Contains(str, "https") &&
		strings.Contains(str, "EOF")

	// In addition to the above error we're getting a "context deadline exceeded" generic error
	// Example error:
	// Internal error occurred: failed calling webhook "inmemorychannel.eventing.knative.dev": failed to call webhook: Post "https://inmemorychannel-webhook.knative-eventing-9zkohswwf9.svc:443/defaulting?timeout=10s": context deadline exceeded
	isContextDeadlineExceededError := strings.Contains(str, "webhook") &&
		strings.Contains(str, "https") &&
		strings.Contains(str, "context deadline exceeded")

	return isEOFError || isContextDeadlineExceededError
}
