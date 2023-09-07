package auth

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetAudience returns the audience string for the given object in the format <group>/<kind>/<namespace>/<name>
func GetAudience(gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) string {
	aud := fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Kind, objectMeta.Namespace, objectMeta.Name)

	return strings.ToLower(aud)
}
