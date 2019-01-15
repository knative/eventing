package testing

import (
	"os"

	"github.com/knative/eventing/pkg/system"
)

func init() {
	os.Setenv(system.NamespaceEnvKey, "knative-testing")
}
