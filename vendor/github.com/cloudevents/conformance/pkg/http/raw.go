/*
 Copyright 2022 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package http

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
)

type Raw struct {
	Out  io.Writer
	Port int `envconfig:"PORT" default:"8080"`
}

func (raw *Raw) Do() error {
	_, _ = fmt.Fprintf(raw.Out, "listening on :%d\n", raw.Port)
	return http.ListenAndServe(fmt.Sprintf(":%d", raw.Port), raw)
}
func (raw *Raw) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if reqBytes, err := httputil.DumpRequest(r, true); err == nil {
		_, _ = fmt.Fprintf(raw.Out, "%+v\n", string(reqBytes))
	} else {
		_, _ = fmt.Fprintf(raw.Out, "Failed to call DumpRequest: %s\n", err)
	}
	_, _ = fmt.Fprintln(raw.Out, "================")
}
