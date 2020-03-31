package apiserver

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
)

type ResourceWatch struct {
	// GVR the group version resource of the resource to watch.
	GVR schema.GroupVersionResource `json:"gvr"`

	// LabelSelector filters this source to objects to those resources pass the
	// label selector.
	// +optional
	LabelSelector string `json:"selector,omitempty"`
}

type Config struct {
	// Namespace specifies the namespace that Resources[] exist.
	// +required
	Namespace string `json:"namespace"`

	// Resource is the resource this source will track and send related
	// lifecycle events from the Kubernetes ApiServer.
	// +required
	Resources []ResourceWatch `json:"resources"`

	// ResourceOwner is an additional filter to only track resources that are
	// owned by a specific resource type. If ResourceOwner matches Resources[n]
	// then Resources[n] is allowed to pass the ResourceOwner filter.
	// +optional
	ResourceOwner *v1alpha2.APIVersionKind `json:"owner,omitempty"`

	// EventMode controls the format of the event.
	// `Reference` sends a dataref event type for the resource under watch.
	// `Resource` send the full resource lifecycle event.
	// Defaults to `Reference`
	// +optional
	EventMode string `json:"mode,omitempty"`
}
