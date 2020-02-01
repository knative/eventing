/*
Copyright 2020 The Kubernetes Authors.

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

package generators

import (
	"io"

	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
	"k8s.io/klog"
)

// reconcilerReconcilerStubGenerator produces a file of the stub of how to
// implement the reconciler.
type reconcilerReconcilerStubGenerator struct {
	generator.DefaultGen
	outputPackage string
	imports       namer.ImportTracker
	filtered      bool

	reconcilerPkg string
}

var _ generator.Generator = (*reconcilerReconcilerStubGenerator)(nil)

func (g *reconcilerReconcilerStubGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// We generate a single client, so return true once.
	if !g.filtered {
		g.filtered = true
		return true
	}
	return false
}

func (g *reconcilerReconcilerStubGenerator) Namers(c *generator.Context) namer.NameSystems {
	return namer.NameSystems{
		"raw": namer.NewRawNamer(g.outputPackage, g.imports),
	}
}

func (g *reconcilerReconcilerStubGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *reconcilerReconcilerStubGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Infof("processing type %v", t)

	m := map[string]interface{}{
		"type":            t,
		"reconcilerEvent": c.Universe.Type(types.Name{Package: "knative.dev/pkg/reconciler", Name: "Event"}),
		"reconcilerCore": c.Universe.Type(types.Name{
			Package: g.reconcilerPkg,
			Name:    "Core",
		}),
		"reconcilerInterface": c.Universe.Type(types.Name{
			Package: g.reconcilerPkg,
			Name:    "Interface",
		}),
	}

	sw.Do(reconcilerReconcilerStub, m)

	return sw.Error()
}

var reconcilerReconcilerStub = `
// TODO: PLEASE COPY AND MODIFY THIS FILE AS A STARTING POINT

// Reconciler implements controller.Reconciler for {{.type|public}} resources.
type Reconciler struct {
	{{.reconcilerCore|raw}}

	// TODO: add additional requirements here.
}

// Check that our Reconciler implements Interface
var _ {{.reconcilerInterface|raw}} = (*Reconciler)(nil)

// SetCore implements Interface.SetCore.
func (r *Reconciler) SetCore(core *{{.reconcilerCore|raw}}) {
	r.Core = *core
}

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, o *{{.type|raw}}) {{.reconcilerEvent|raw}} {
	if o.GetDeletionTimestamp() != nil {
		// Check for a DeletionTimestamp.  If present, elide the normal reconcile logic.
		// When a controller needs finalizer handling, it would go here.
		return nil
	}	
	o.Status.InitializeConditions()

	// TODO: add custom reconciliation logic here.

	o.Status.ObservedGeneration = o.Generation
	return nil
}

`
