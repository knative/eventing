/*
Copyright 2023 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package eventingtls_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/configmap"
)

func TestStartServers(t *testing.T) {
	testCases := map[string]struct {
		transportEncryption feature.Flag
		handleHttp          bool
		handleHttps         bool
	}{
		"disabled: should only support http": {
			transportEncryption: feature.Disabled,
			handleHttp:          true,
			handleHttps:         false,
		},
		"permissive: should support both http and https": {
			transportEncryption: feature.Permissive,
			handleHttp:          true,
			handleHttps:         true,
		},
		"strict: should only support https": {
			transportEncryption: feature.Strict,
			handleHttp:          false,
			handleHttps:         true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ctx, cancelFunc := context.WithCancel(context.TODO())
			httpReceiver := kncloudevents.NewHTTPEventReceiver(0, kncloudevents.WithDrainQuietPeriod(time.Millisecond))
			httpsReceiver := kncloudevents.NewHTTPEventReceiver(0, kncloudevents.WithDrainQuietPeriod(time.Millisecond))
			errChan := make(chan error)

			cmw := newFeatureCMW(tc.transportEncryption)
			sm, err := eventingtls.NewServerManager(ctx, httpReceiver, httpsReceiver, &basicHandler{}, cmw)
			assert.NoError(t, err)
			go func() {
				errChan <- sm.StartServers(ctx)
			}()

			<-httpReceiver.Ready
			<-httpsReceiver.Ready

			client := &http.Client{}
			httpAddr := "http://" + httpReceiver.GetAddr()
			httpsAddr := "http://" + httpsReceiver.GetAddr()

			httpReq, err := http.NewRequest("GET", httpAddr, nil)
			assert.NoError(t, err)
			httpsReq, err := http.NewRequest("GET", httpsAddr, nil)
			assert.NoError(t, err)

			httpResp, err := client.Do(httpReq)
			assert.NoError(t, err)
			if tc.handleHttp {
				assert.Equal(t, http.StatusOK, httpResp.StatusCode)
			} else {
				assert.Equal(t, http.StatusNotFound, httpResp.StatusCode)
			}

			httpsResp, err := client.Do(httpsReq)
			assert.NoError(t, err)
			if tc.handleHttps {
				assert.Equal(t, http.StatusOK, httpsResp.StatusCode)
			} else {
				assert.Equal(t, http.StatusNotFound, httpsResp.StatusCode)
			}

			// stop servers and ensure no errors for http/https
			cancelFunc()
			assert.NoError(t, <-errChan)
		})
	}
}

func TestStartServersHttpError(t *testing.T) {
	ctx := context.TODO()
	receiver := kncloudevents.NewHTTPEventReceiver(0, kncloudevents.WithDrainQuietPeriod(time.Millisecond))
	cmw := newFeatureCMW(feature.Disabled)

	// httpReceiver set to nil
	_, err := eventingtls.NewServerManager(ctx, nil, receiver, &basicHandler{}, cmw)
	assert.Error(t, err)

	// httpsReceiver set to nil
	_, err = eventingtls.NewServerManager(ctx, receiver, nil, &basicHandler{}, cmw)
	assert.Error(t, err)
}

type basicHandler struct{}

func (bh *basicHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func newFeatureCMW(value feature.Flag) configmap.Watcher {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: feature.FlagsConfigName,
		},
		Data: map[string]string{
			feature.TransportEncryption: string(value),
		},
	}

	return configmap.NewStaticWatcher(cm)
}
