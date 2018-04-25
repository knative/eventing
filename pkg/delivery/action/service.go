package action

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/elafros/eventing/pkg/event"
	"github.com/golang/glog"
)

const (
	// ServiceActionType is the expected Bind Processor name to
	// cause Events to be sent to a K8S service.
	ServiceActionType = "Service"
)

// serviceAction sends Events to a K8S service.
type serviceAction struct {
	httpclient *http.Client
}

// NewServiceAction creates an Action object that sends Events to K8S services.
func NewServiceAction(httpclient *http.Client) Action {
	return &serviceAction{httpclient: httpclient}
}

// SendEvent will POST the event spec to the root URI of the elafros route.
func (a *serviceAction) SendEvent(name string, data interface{}, context *event.Context) (interface{}, error) {
	glog.Infof("Sending event %s to K8S service %s", context.EventID, name)
	var namespace, service string
	parts := strings.Split(name, "/")
	if len(parts) != 2 {
		// TODO(inlined): for compatibility with the github demo we implicitly pick up the namespace
		// of the Bind resource. Should ensure that we'll always get routes in this form.
		namespace, service = "default", name
	} else {
		namespace, service = parts[0], parts[1]
	}

	addr := fmt.Sprintf("http://%s.%s.svc.cluster.local", service, namespace)
	glog.Info("Sending event", context.EventID, "to K8S service at", addr)
	req, err := event.NewRequest(addr, data, *context)
	if err != nil {
		glog.Errorf("Failed to marshal event: %s", err)
		return nil, err
	}

	res, err := a.httpclient.Do(req)
	if err != nil {
		glog.Errorf("Failed to send event to webhook %s; err=%s", addr, err)
		return nil, err
	}
	// TODO: Standard handling of non-200 responses as errors.
	if res.StatusCode/100 != 2 {
		glog.Errorf("Got unsuccessful response code %d from %s", res.StatusCode, addr)
	}

	// TODO: Return response data so that it may be forwarded in chained Binds.
	glog.Infof("Sent event to %s; got response %s", addr, res.Status)
	return nil, nil
}
