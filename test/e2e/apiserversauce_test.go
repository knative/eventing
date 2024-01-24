package e2e

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/feature"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/reconciler/apiserversource/resources"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
)

var (
	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Channel",
			APIVersion: "messaging.knative.dev/v1",
		},
	}
	sinkDNS      = "sink.mynamespace.svc." + network.GetClusterDomainName()
	sinkURI      = apis.HTTP(sinkDNS)
	sinkURL      = apis.HTTP(sinkDNS)
	sinkAudience = "sink-oidc-audience"
)

const (
	image      = "github.com/knative/test/image"
	sourceName = "test-apiserver-source"
	sourceUID  = "1234"
	sinkName   = "testsink"
)

func TestAPIServerSauce(t *testing.T) {
	ctx := context.Background()

	c := testlib.Setup(t, true)

	defer testlib.TearDown(c)

	t.Run("should deploy at the node set in the config-features", func(t *testing.T) {
		source := &v1.ApiServerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example-deployment",
				Namespace: c.Namespace,
				UID:       sourceUID,
			},
			Spec: v1.ApiServerSourceSpec{
				Resources: []sourcesv1.APIVersionKindSelector{{
					APIVersion: "v1",
					Kind:       "Namespace",
				}},

				SourceSpec: duckv1.SourceSpec{Sink: sinkDest},
			},
		}

		nodeList, err := c.Kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		assert.Nil(t, err)

		var selectedNodeName string
		if len(nodeList.Items) > 0 {
			selectedNode := nodeList.Items[rand.Intn(len(nodeList.Items))]
			selectedNodeName = selectedNode.Name
		} else {
			assert.Fail(t, "no nodes found")
		}

		node, err := c.Kube.CoreV1().Nodes().Get(ctx, selectedNodeName, metav1.GetOptions{})
		assert.Nil(t, err)

		node.Labels["testkey"] = "testvalue"

		_, err = c.Kube.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		assert.Nil(t, err)

		configData := map[string]string{
			"apiserversources.nodeselector.testkey": "testvalue",
		}

		cm := c.CreateConfigMapOrFail("config-features", c.Namespace, configData)

		assert.NotNil(t, cm)

		flags, err := feature.NewFlagsConfigFromConfigMap(cm)

		assert.Nil(t, err)

		ctx = feature.ToContext(ctx, flags)

		featureFlags := feature.FromContext(ctx)

		args := resources.ReceiveAdapterArgs{
			Image:        image,
			Source:       source,
			Labels:       resources.Labels(sourceName),
			SinkURI:      sinkURI.String(),
			Configs:      &reconcilersource.EmptyVarsGenerator{},
			Namespaces:   []string{c.Namespace},
			Audience:     &sinkAudience,
			NodeSelector: featureFlags.NodeSelector(),
		}

		dep, err := resources.MakeReceiveAdapter(&args)

		assert.Nil(t, err)

		c.CreateDeploymentOrFail(dep)

		newDep, err := c.Kube.AppsV1().Deployments(c.Namespace).Get(ctx, dep.ObjectMeta.Name, metav1.GetOptions{})

		assert.Nil(t, err)

		assert.Equal(t, newDep.Spec.Template.Spec.NodeSelector, featureFlags.NodeSelector())
	})
}
