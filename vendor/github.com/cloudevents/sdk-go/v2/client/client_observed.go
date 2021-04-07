package client

// NewObserved produces a new client with the provided transport object and applied
// client options.
// Deprecated: This now has the same behaviour of New, and will be removed in future releases.
// As New, you must provide the observability service to use.
var NewObserved = New
