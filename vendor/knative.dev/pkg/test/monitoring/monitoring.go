/*
Copyright 2019 The Knative Authors

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

package monitoring

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"knative.dev/pkg/test/logging"
)

// CheckPortAvailability checks to see if the port is available on the machine.
func CheckPortAvailability(port int) error {
	server, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		// Port is likely taken
		return err
	}
	server.Close()

	return nil
}

// GetPods retrieves the current existing podlist for the app in monitoring namespace
// This uses app=<app> as labelselector for selecting pods
func GetPods(kubeClientset *kubernetes.Clientset, app, namespace string) (*v1.PodList, error) {
	pods, err := kubeClientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", app)})
	if err == nil && len(pods.Items) == 0 {
		err = fmt.Errorf("no %s Pod found on the cluster. Ensure monitoring is switched on for your Knative Setup", app)
	}

	return pods, err
}

// PortForward sets up local port forward to the pod specified by the "app" label in the given namespace
// To close the port forwarding, just close the channel
func PortForward(logf logging.FormatLogger, config *rest.Config, clientSet *kubernetes.Clientset, pod *v1.Pod, localPort, remotePort int) (chan struct{}, error) {
	req := clientSet.RESTClient().Post().Resource("pods").Namespace(pod.Namespace).Name(pod.Name).SubResource("portforward")
	portForwardUrl := req.URL()
	if !strings.HasPrefix(portForwardUrl.Path, "/api/v1") {
		portForwardUrl.Path = "/api/v1" + portForwardUrl.Path
	}

	stopChan := make(chan struct{})
	readyChan := make(chan struct{})

	transport, upgrader, err := spdy.RoundTripperFor(config)
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, portForwardUrl)
	fw, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf("%d:%d", localPort, remotePort)},
		stopChan,
		readyChan,
		logging.NewLoggerWriter("port-forward-out-"+pod.Name+": ", logf),
		logging.NewLoggerWriter("port-forward-err-"+pod.Name+": ", logf),
	)
	if err != nil {
		return nil, err
	}
	go func() {
		err := fw.ForwardPorts()
		if err != nil {
			log.Fatalf("Error opening the port forward for pod %s in ns %s with ports %d:%d, cause: %v", pod.Name, pod.Namespace, localPort, remotePort, err)
		}
	}()

	<-readyChan
	logf("Started port forwarding for pod %s in ns %s with ports %d:%d", pod.Name, pod.Namespace, localPort, remotePort)
	return stopChan, nil
}
