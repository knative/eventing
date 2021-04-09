/*
Copyright 2021 The Knative Authors

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

package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"knative.dev/hack/schema/docs"
	"knative.dev/hack/schema/registry"
	"knative.dev/hack/schema/schema"
)

// New creates a new schema cli command set.
func New(root string) *cobra.Command {
	docs.SetRoot(root)

	var cmd = &cobra.Command{
		Use:   "schema",
		Short: "Interact with the schema of build in types.",
	}

	addDumpCmd(cmd)

	return cmd
}

func addDumpCmd(root *cobra.Command) {
	var kind string

	var cmd = &cobra.Command{
		Use:   "dump <kind>",
		Short: "Dump the raw output of schemas of known kinds.",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validation
			kind = args[0]
			if t := registry.TypeFor(kind); t == nil {
				known := registry.Kinds()
				return fmt.Errorf("unknown Kind: %s, expected one of [%s]", kind, strings.Join(known, ", "))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			s := schema.GenerateForType(registry.TypeFor(kind))
			enc := yaml.NewEncoder(os.Stdout)
			enc.SetIndent(2)
			err := enc.Encode(s)
			if err != nil {
				return err
			}
			return nil
		},
	}
	// TODO: add support for versions, etc.

	root.AddCommand(cmd)
}
