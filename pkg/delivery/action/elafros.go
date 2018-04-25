package action

import (
	"fmt"
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/elafros/eventing/pkg/event"
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
)

const (
	// ElafrosActionType is the expected Bind Processor name to
	// cause Events to be sent to an Elafros Route.
	ElafrosActionType = "elafros.dev/Route"
)

// elafrosAction sends Events to an Elafros Route.
type elafrosAction struct {
	kubeclientset kubernetes.Interface
	httpclient    *http.Client
}

// NewElafrosAction creates an Action object that sends Events to Elafros Routes.
func NewElafrosAction(kubeclientset kubernetes.Interface, httpclient *http.Client) Action {
	return &elafrosAction{kubeclientset: kubeclientset, httpclient: httpclient}
}

// SendEvent will POST the event spec to the root URI of the elafros route.
func (a *elafrosAction) SendEvent(name string, data interface{}, context *event.Context) (interface{}, error) {
	glog.Infof("Sending event %s to ELA route %s", context.EventID, name)
	var namespace, route string
	parts := strings.Split(name, "/")
	if len(parts) != 2 {
		// TODO(inlined): for compatibility with the github demo we implicitly pick up the namespace
		// of the Bind resource. Should ensure that we'll always get routes in this form.
		namespace, route = "default", name
	} else {
		namespace, route = parts[0], parts[1]
	}

	domain, err := getDomainSuffixFromElaConfig(a.kubeclientset)
	if err != nil {
		glog.Error("Could not look up Elafros domain")
		return nil, err
	}

	addr := fmt.Sprintf("http://%s.%s.%s/", route, namespace, domain)
	glog.Info("Sending event", context.EventID, "to ELA route at", addr)
	req, err := event.NewRequest(addr, data, *context)
	if err != nil {
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

func getDomainSuffixFromElaConfig(cl kubernetes.Interface) (string, error) {
	const name = "ela-config"
	const namespace = "ela-system"
	c, err := cl.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	domainSuffix, ok := c.Data["domainSuffix"]
	if !ok {
		return "", fmt.Errorf("cannot find domainSuffix in %v: ConfigMap.Data is %#v", name, c.Data)
	}
	return domainSuffix, nil
}
