package sdk

import (
	"context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Reconciler interface {
	// takes a byte[] and VersionKind
	Convert(context.Context, schema.GroupVersionKind, []byte) (runtime.Object, error)
	// Takes in the parent and children, returns the parent status and requested children state.
	Reconcile(context.Context, runtime.Object, []runtime.Object) (interface{}, []runtime.Object, error)
}

type Controller interface {
	Handle(context.Context, *Request) (*Response, error)
}

type Request struct {
	// Controller is the overall context for the reconciler.
	Controller unstructured.Unstructured `json:"controller"`
	// Parent is the object under reconciliation.
	Parent unstructured.Unstructured `json:"parent"`
	// Children is a map where the key is gvk and the value is a ChildrenGroup
	Children   map[string]ChildrenGroup `json:"children"`
	Finalizing bool                     `json:"finalizing"`
}

// ChildrenGroup is a map of resource names to a resource.
type ChildrenGroup map[string]unstructured.Unstructured

type Response struct {
	Status   unstructured.Unstructured   `json:"status"`
	Children []unstructured.Unstructured `json:"children"`
}
