package conformance

import (
	"knative.dev/eventing/test/conformance/helpers"
	"knative.dev/eventing/test/lib"
	"testing"
)

func TestSourceCRDMetadata(t *testing.T) {
	helpers.SourceCRDMetadataTestHelperWithChannelTestRunner(t, sourcesTestRunner, lib.SetupClientOptionNoop)
}

