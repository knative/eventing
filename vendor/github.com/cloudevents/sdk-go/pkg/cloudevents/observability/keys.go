package observability

import (
	"go.opencensus.io/tag"
)

var (
	KeyMethod, _ = tag.NewKey("method")
	KeyResult, _ = tag.NewKey("result")
)

const (
	ResultError = "error"
	ResultOK    = "success"
)
