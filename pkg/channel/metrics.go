package channel

import "go.opencensus.io/tag"

const (
	// LabelUniqueName is the label for the unique name per stats_reporter instance.
	LabelUniqueName = "unique_name"

	// LabelContainerName is the label for the immutable name of the container.
	LabelContainerName = "container_name"
)

var (
	ContainerTagKey = tag.MustNewKey(LabelContainerName)
	UniqueTagKey    = tag.MustNewKey(LabelUniqueName)
)
