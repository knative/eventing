/*
Copyright 2020 The Knative Authors

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

package crstatusevent

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
)

type crStatusEvent struct {
	Recorder      record.EventRecorder
	Logf          func(format string, args ...interface{})
	source        runtime.Object
	component     string
	kubeEventSink *record.EventSink
}
type CRStatusEventClient struct {
	isEnabledVar bool
	m            sync.RWMutex
}

func GetDefaultClient() *CRStatusEventClient {
	return &CRStatusEventClient{}
}

func NewCRStatusEventClient(metricMap map[string]string) *CRStatusEventClient {
	if metricMap == nil {
		return nil
	}

	ret := &CRStatusEventClient{}
	if metricMap["sink-event-error-reporting.enable"] == "true" {
		ret.isEnabledVar = true
	}
	return ret

}

// UpdateFromConfigMap returns an updater function to be used
// with ConfigWatcher, that given an observability ConfigMap
// updates the client's Event Recorder configuration.
func UpdateFromConfigMap(client *CRStatusEventClient) func(configMap *corev1.ConfigMap) {
	return func(cm *corev1.ConfigMap) {
		if cm != nil && cm.Data != nil && cm.Data["sink-event-error-reporting.enable"] != "" {
			client.m.Lock()
			defer client.m.Unlock()
			client.isEnabledVar, _ = strconv.ParseBool(cm.Data["sink-event-error-reporting.enable"])
		}
	}
}

var contextkey struct{}

func ContextWithCRStatus(ctx context.Context, kubeEventSink *record.EventSink, component string, source runtime.Object, logf func(format string, args ...interface{})) context.Context {

	return context.WithValue(ctx, contextkey, &crStatusEvent{
		component:     component,
		Logf:          logf,
		kubeEventSink: kubeEventSink,
		source:        source.DeepCopyObject(),
	})
}

func fromContext(ctx context.Context) (*crStatusEvent, bool) {
	crStatusEvent, ok := ctx.Value(contextkey).(*crStatusEvent)
	return crStatusEvent, ok
}

func (c *CRStatusEventClient) ReportCRStatusEvent(ctx context.Context, result protocol.Result) {
	c.m.RLock()
	defer c.m.RUnlock()

	if !c.isEnabledVar {
		return
	}

	if fromContext, ok := fromContext(ctx); !ok {
		return
	} else {
		fromContext.createEvent(ctx, result)
	}
}

func (a *crStatusEvent) getRecorder(ctx *context.Context, kubeEventSink *record.EventSink, logf func(format string, args ...interface{}), component string) record.EventRecorder {
	if a.Recorder == nil {
		eventBroadcaster := record.NewBroadcaster()

		watches := []watch.Interface{
			eventBroadcaster.StartLogging(logf),
			eventBroadcaster.StartRecordingToSink(
				*kubeEventSink),
		}
		a.Recorder = eventBroadcaster.NewRecorder(
			scheme.Scheme, corev1.EventSource{Component: component})
		go func() {
			<-(*ctx).Done()
			a.Recorder = nil
			for _, w := range watches {
				w.Stop()
			}
		}()
	}
	return a.Recorder
}

func (a *crStatusEvent) createEvent(ctx context.Context, result protocol.Result) {

	reason := "500"
	msg := "Error sending cloud event to sink."

	var res *http.Result
	if !cloudevents.ResultAs(result, &res) {
		a.Logf("Failed converting to http Result &v", result)
		msg += result.Error()
	} else {
		if res.StatusCode < 400 { // don't report
			return
		}
		reason = strconv.Itoa(res.StatusCode)
		if res.Format != "" && res.Format != "%w" { // returns '"%w" but this does not format
			msg += " " + fmt.Sprintf(res.Format, res.Args...)
		} else if res.Args != nil && len(res.Args) > 0 {
			if m, ok := res.Args[0].(*protocol.Receipt); ok {
				if m.Err != nil {
					msg += " " + m.Err.Error() // add any error message if it's there.
				}
			}
		}
	}

	recorder := a.getRecorder(&ctx, a.kubeEventSink, a.Logf, a.component)
	recorder.Eventf(a.source, corev1.EventTypeWarning, "SinkSendFailed", reason+" "+msg)

}
