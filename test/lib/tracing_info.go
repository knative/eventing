package lib

import (
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	configtracing "knative.dev/pkg/tracing/config"
)

func (c *Client) GetTracingConfigOrFail() *configtracing.Config {
	cm, err := c.Kube.Kube.CoreV1().ConfigMaps("knative-eventing").Get("config-tracing", metav1.GetOptions{})
	if err != nil {
		c.T.Fatalf("Error while retrieving the config-tracing config map: %+v", errors.WithStack(err))
	}

	config, err := configtracing.NewTracingConfigFromConfigMap(cm)
	if err != nil {
		c.T.Fatalf("Error while parsing the config-tracing config map: %+v", errors.WithStack(err))
	}

	return config
}
