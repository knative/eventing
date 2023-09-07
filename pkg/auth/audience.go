package auth

import (
	"fmt"
	"strings"

	"knative.dev/pkg/kmeta"
)

// GetAudience returns the audience string for the given object in the format <group>/<kind>/<namespace>/<name>
func GetAudience(obj kmeta.Accessor) string {
	aud := fmt.Sprintf("%s/%s/%s/%s", obj.GroupVersionKind().Group, obj.GroupVersionKind().Kind, obj.GetNamespace(), obj.GetName())

	return strings.ToLower(aud)
}
