package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/test"
	pkgTest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	servingV1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacV1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	// Mysteriously required to support GCP auth (required by k8s libs).
	// Apparently just importing it is enough. @_@ side effects @_@.
	// https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	defaultNamespaceName = "e2etestfn3"
	testNamespace        = "e2etest"
	interval             = 1 * time.Second
	timeout              = 1 * time.Minute
)

// Setup creates the client objects needed in the e2e tests.
func Setup(t *testing.T, logger *logging.BaseLogger) (*test.Clients, *test.Cleaner) {
	if pkgTest.Flags.Namespace == "" {
		pkgTest.Flags.Namespace = defaultNamespaceName
	}

	clients, err := test.NewClients(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster,
		pkgTest.Flags.Namespace)
	if err != nil {
		t.Fatalf("Couldn't initialize clients: %v", err)
	}
	cleaner := test.NewCleaner(logger, clients.Dynamic)

	return clients, cleaner
}

// TearDown will delete created names using clients.
func TearDown(clients *test.Clients, cleaner *test.Cleaner, logger *logging.BaseLogger) {
	cleaner.Clean(true)

	// There seems to be an Istio bug where if we delete / create
	// VirtualServices too quickly we will hit pro-longed "No health
	// upstream" causing timeouts.  Adding this small sleep to
	// sidestep the issue.
	//
	// TODO(#1376):  Fix this when upstream fix is released.
	logger.Info("Sleeping for 20 seconds after clean to avoid hitting issue in #1376")
	time.Sleep(20 * time.Second)
}

// CreateRouteAndConfig will create Route and Config objects using clients.
// The Config object will serve requests to a container started from the image at imagePath.
func CreateRouteAndConfig(clients *test.Clients, logger *logging.BaseLogger, cleaner *test.Cleaner, name string, imagePath string) error {
	configurations := clients.Serving.ServingV1alpha1().Configurations(pkgTest.Flags.Namespace)
	config, err := configurations.Create(
		test.Configuration(name, pkgTest.Flags.Namespace, imagePath))
	if err != nil {
		return err
	}
	cleaner.Add(servingV1alpha1.SchemeGroupVersion.Group, servingV1alpha1.SchemeGroupVersion.Version, "configurations", pkgTest.Flags.Namespace, config.ObjectMeta.Name)

	routes := clients.Serving.ServingV1alpha1().Routes(pkgTest.Flags.Namespace)
	route, err := routes.Create(
		test.Route(name, pkgTest.Flags.Namespace, name))
	if err != nil {
		return err
	}
	cleaner.Add(servingV1alpha1.SchemeGroupVersion.Group, servingV1alpha1.SchemeGroupVersion.Version, "routes", pkgTest.Flags.Namespace, route.ObjectMeta.Name)
	return nil
}

// WithRouteReady will create Route and Config objects and wait until they're ready.
func WithRouteReady(clients *test.Clients, logger *logging.BaseLogger, cleaner *test.Cleaner, name string, imagePath string) error {
	err := CreateRouteAndConfig(clients, logger, cleaner, name, imagePath)
	if err != nil {
		return err
	}
	routes := clients.Serving.ServingV1alpha1().Routes(pkgTest.Flags.Namespace)
	if err := test.WaitForRouteState(routes, name, test.IsRouteReady, "RouteIsReady"); err != nil {
		return err
	}
	return nil
}

// CreateKubernetesEventSource creates a KubernetesEventSource
func CreateKubernetesEventSource(clients *test.Clients, source *sourcesv1alpha1.KubernetesEventSource, logger *logging.BaseLogger, cleaner *test.Cleaner) error {
	k8sSources := clients.Sources.SourcesV1alpha1().KubernetesEventSources(pkgTest.Flags.Namespace)
	res, err := k8sSources.Create(source)
	if err != nil {
		return err
	}
	cleaner.Add(sourcesv1alpha1.SchemeGroupVersion.Group, sourcesv1alpha1.SchemeGroupVersion.Version, "kuberneteseventsources", pkgTest.Flags.Namespace, res.ObjectMeta.Name)
	return nil
}

// CreateChannel will create a Channel
func CreateChannel(clients *test.Clients, channel *v1alpha1.Channel, logger *logging.BaseLogger, cleaner *test.Cleaner) error {
	channels := clients.Eventing.EventingV1alpha1().Channels(pkgTest.Flags.Namespace)
	res, err := channels.Create(channel)
	if err != nil {
		return err
	}
	cleaner.Add(v1alpha1.SchemeGroupVersion.Group, v1alpha1.SchemeGroupVersion.Version, "channels", pkgTest.Flags.Namespace, res.ObjectMeta.Name)
	return nil
}

// CreateSubscription will create a Subscription
func CreateSubscription(clients *test.Clients, subs *v1alpha1.Subscription, logger *logging.BaseLogger, cleaner *test.Cleaner) error {
	subscriptions := clients.Eventing.EventingV1alpha1().Subscriptions(pkgTest.Flags.Namespace)
	res, err := subscriptions.Create(subs)
	if err != nil {
		return err
	}
	cleaner.Add(v1alpha1.SchemeGroupVersion.Group, v1alpha1.SchemeGroupVersion.Version, "subscriptions", pkgTest.Flags.Namespace, res.ObjectMeta.Name)
	return nil
}

// CreateServiceAccount will create a service account
func CreateServiceAccount(clients *test.Clients, sa *corev1.ServiceAccount, logger *logging.BaseLogger, cleaner *test.Cleaner) error {
	sas := clients.Kube.Kube.CoreV1().ServiceAccounts(pkgTest.Flags.Namespace)
	res, err := sas.Create(sa)
	if err != nil {
		return err
	}
	cleaner.Add(corev1.SchemeGroupVersion.Group, corev1.SchemeGroupVersion.Version, "serviceaccounts", pkgTest.Flags.Namespace, res.ObjectMeta.Name)
	return nil
}

// CreateClusterRoleBinding will create a service account binding
func CreateClusterRoleBinding(clients *test.Clients, crb *rbacV1beta1.ClusterRoleBinding, logger *logging.BaseLogger, cleaner *test.Cleaner) error {
	clusterRoleBindings := clients.Kube.Kube.RbacV1beta1().ClusterRoleBindings()
	res, err := clusterRoleBindings.Create(crb)
	if err != nil {
		return err
	}
	cleaner.Add(rbacV1beta1.SchemeGroupVersion.Group, rbacV1beta1.SchemeGroupVersion.Version, "clusterrolebindings", "", res.ObjectMeta.Name)
	return nil
}

// CreateServiceAccountAndBinding creates both ServiceAccount and ClusterRoleBinding with default
// cluster-admin role
func CreateServiceAccountAndBinding(clients *test.Clients, name string, logger *logging.BaseLogger, cleaner *test.Cleaner) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespaceName,
		},
	}
	err := CreateServiceAccount(clients, sa, logger, cleaner)
	if err != nil {
		return err
	}
	crb := &rbacV1beta1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "e2e-tests-admin",
		},
		Subjects: []rbacV1beta1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      name,
				Namespace: defaultNamespaceName,
			},
		},
		RoleRef: rbacV1beta1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	err = CreateClusterRoleBinding(clients, crb, logger, cleaner)
	if err != nil {
		return err
	}
	return nil
}

// CreatePod will create a Pod
func CreatePod(clients *test.Clients, pod *corev1.Pod, logger *logging.BaseLogger, cleaner *test.Cleaner) error {
	pods := clients.Kube.Kube.CoreV1().Pods(pod.GetNamespace())
	res, err := pods.Create(pod)
	if err != nil {
		return err
	}
	cleaner.Add(corev1.SchemeGroupVersion.Group, corev1.SchemeGroupVersion.Version, "pods", res.ObjectMeta.Namespace, res.ObjectMeta.Name)
	return nil
}

// PodLogs returns Pod logs for given Pod and Container
func PodLogs(clients *test.Clients, podName string, containerName string, logger *logging.BaseLogger) ([]byte, error) {
	pods := clients.Kube.Kube.CoreV1().Pods(pkgTest.Flags.Namespace)
	podList, err := pods.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, pod := range podList.Items {
		if strings.Contains(pod.Name, podName) {
			result := pods.GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: containerName,
			}).Do()
			return result.Raw()
		}
	}
	return nil, fmt.Errorf("Could not find logs for %s/%s", podName, containerName)
}

// WaitForLogContent waits until logs for given Pod/Container include the given content.
// If the content is not present within timeout it returns error.
func WaitForLogContent(clients *test.Clients, logger *logging.BaseLogger, podName string, containerName string, content string) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		logs, err := PodLogs(clients, podName, containerName, logger)
		if err != nil {
			return true, err
		}
		return strings.Contains(string(logs), content), nil
	})
}

// WaitForAllPodsRunning will wait until all pods in the given namespace are running
func WaitForAllPodsRunning(clients *test.Clients, logger *logging.BaseLogger, namespace string) error {
	if err := pkgTest.WaitForPodListState(clients.Kube, test.PodsRunning, "PodsAreRunning", namespace); err != nil {
		return err
	}
	return nil
}

// ImagePath is a helper function to prefix image name with repo and suffix with tag
func ImagePath(name string) string {
	return fmt.Sprintf("%s/%s:%s", test.EventingFlags.DockerRepo, name, test.EventingFlags.Tag)
}
