/*
 Copyright 2024 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package extensions

import (
	"github.com/cloudevents/sdk-go/v2/event"
	"net/url"
)

const DataRefExtensionKey = "dataref"

// DataRefExtension represents the CloudEvents Dataref (claim check pattern)
// extension for cloudevents contexts,
// See https://github.com/cloudevents/spec/blob/main/cloudevents/extensions/dataref.md
// for more info
type DataRefExtension struct {
	DataRef string `json:"dataref"`
}

// AddDataRefExtension adds the dataref attribute to the cloudevents context
func AddDataRefExtension(e *event.Event, dataRef string) error {
	if _, err := url.Parse(dataRef); err != nil {
		return err
	}
	e.SetExtension(DataRefExtensionKey, dataRef)
	return nil
}

// GetDataRefExtension returns any dataref attribute present in the
// cloudevent event/context and a bool to indicate if it was found.
// If not found, the DataRefExtension.DataRef value will be ""
func GetDataRefExtension(e event.Event) (DataRefExtension, bool) {
	if dataRefValue, ok := e.Extensions()[DataRefExtensionKey]; ok {
		dataRefStr, _ := dataRefValue.(string)
		return DataRefExtension{DataRef: dataRefStr}, true
	}
	return DataRefExtension{}, false
}
