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

// ElafrosAction sends Events to an Elafros Route.
type ElafrosAction struct {
	kubeclientset kubernetes.Interface
	httpclient    *http.Client
}

// NewElafrosAction creates an Action object that sends Events to Elafros Routes.
func NewElafrosAction(kubeclientset kubernetes.Interface, httpclient *http.Client) *ElafrosAction {
	return &ElafrosAction{kubeclientset: kubeclientset, httpclient: httpclient}
}

// SendEvent will POST the event spec to the root URI of the elafros route.
func (a *ElafrosAction) SendEvent(name string, data interface{}, context *event.Context) (interface{}, error) {
	parts := strings.Split(name, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("Expected elafros route '%s' to be in the form '<namespace>/<route>'", name)
	}
	route, namespace := parts[1], parts[0]

	domain, err := getDomainSuffixFromElaConfig(a.kubeclientset)
	if err != nil {
		glog.Error("Could not look up Elafros domain")
		return nil, err
	}

	addr := fmt.Sprintf("http://%s.%s.%s", route, namespace, domain)
	req, err := event.NewRequest(addr, data, *context)
	if err != nil {
		return nil, err
	}

	if _, err := a.httpclient.Do(req); err != nil {
		return nil, err
	}

	// TODO: non-200 responses as errors. Standard decoding of responses to be forwarded.
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
