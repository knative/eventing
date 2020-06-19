/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package recordevents

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/test/monitoring"

	"knative.dev/pkg/test/logging"

	testlib "knative.dev/eventing/test/lib"
)

// Port for the recordevents pod REST listener
const RecordEventsPort = 8392

// HTTP path for the GetMinMax REST call
const GetMinMaxPath = "/minmax"

// HTTP path for the GetEntry REST call
const GetEntryPath = "/entry/"

// HTTP path for the TrimThrough REST call
const TrimThroughPath = "/trimthrough/"

// On-wire json rest api format for recordevents GetMinMax calls
// sennt to the recordevents pod.
type MinMaxResponse struct {
	MinAvail int
	MaxSeen  int
}

// Structure to hold information about an event seen by recordevents pod.
type EventInfo struct {
	// Set if the http request received by the pod couldn't be decoded or
	// didn't pass validation
	Error string
	// Event received if the cloudevent received by the pod passed validation
	Event *cloudevents.Event
	// HTTPHeaders of the connection that delivered the event
	HTTPHeaders map[string][]string
}

// Pretty print the event. Meant for debugging.  This formats the validation error
// or the full event as appropriate.  This does NOT format the headers.
func (ei *EventInfo) String() string {
	if ei.Event != nil {
		return ei.Event.String()
	} else {
		return fmt.Sprintf("invalid event \"%s\"", ei.Error)
	}
}

// This is mainly used for providing better failure messages
type SearchedInfo struct {
	TotalEvent int
	LastNEvent []EventInfo
}

// Pretty print the SearchedInfor for error messages
func (s *SearchedInfo) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d events seen, last %d events:", s.TotalEvent, len(s.LastNEvent)))
	for _, ei := range s.LastNEvent {
		sb.WriteString(ei.String())
		sb.WriteRune('\n')
	}
	return sb.String()
}

// Connection state for a REST connection to a pod
type eventGetter struct {
	podName       string
	podNamespace  string
	podPort       int
	kubeClientset kubernetes.Interface
	logf          logging.FormatLogger

	host       string
	port       int
	forwardPID int
}

// Creates a forwarded port to the specified recordevents pod and waits until
// it can successfully talk to the REST API.  Times out after timeoutEvRetry
func newEventGetter(podName string, client *testlib.Client, logf logging.FormatLogger) (eventGetterInterface, error) {
	egi := &eventGetter{podName: podName, podNamespace: client.Namespace,
		kubeClientset: client.Kube.Kube, podPort: RecordEventsPort, logf: logf}
	err := egi.forwardPort()
	if err != nil {
		return nil, err
	}

	err = egi.waitTillUp()
	if err != nil {
		return nil, err
	}
	return egi, nil
}

// Get information about the provided podName.  Uses list (rather than get) and
// returns a pod list for compatibility with the monitoring.PortForward
// interface
func (eg *eventGetter) getRunningPodInfo(podName, namespace string) (*v1.PodList, error) {
	pods, err := eg.kubeClientset.CoreV1().Pods(namespace).List(context.Background(),
		metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", podName)})
	if err == nil && len(pods.Items) != 1 {
		err = fmt.Errorf("no %s Pod found on the cluster", podName)
	} else if pods.Items[0].Status.Phase != corev1.PodRunning {
		err = fmt.Errorf("pod %s in state %s, wanted Running", podName,
			pods.Items[0].Status.Phase)
	}

	return pods, err
}

// Try to forward the pod port to a local port somewhere in the range 30000-60000.
// keeps retrying with random ports in that range, timing out after timeoutEvRetry
func (eg *eventGetter) forwardPort() error {
	portRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	portMin := 30000
	portMax := 60000
	var internalErr error

	wait.PollImmediate(minEvRetryInterval, timeoutEvRetry, func() (bool, error) {
		localPort := portMin + portRand.Intn(portMax-portMin)
		if err := monitoring.CheckPortAvailability(localPort); err != nil {
			internalErr = err
			return false, nil
		}
		pods, err := eg.getRunningPodInfo(eg.podName, eg.podNamespace)
		if err != nil {
			internalErr = err
			return false, nil
		}

		pid, err := monitoring.PortForward(eg.logf, pods, localPort, eg.podPort, eg.podNamespace)
		if err != nil {
			internalErr = err
			return false, nil
		}
		internalErr = nil

		eg.forwardPID = pid
		eg.port = localPort
		eg.host = "localhost"
		return true, nil
	})
	if internalErr != nil {
		return fmt.Errorf("timeout forwarding port: %v", internalErr)
	}
	return nil
}

// Return the min available, max seen by the recordevents pod.
// maxRet is the largest event that has ever been seen (whether it's been trimmed
// or not).  minRet is the smallest event still available via Get, or 1+maxRet if
// no events are available.  maxRet starts at 0 when no events have been seen.
func (eg *eventGetter) getMinMax() (minRet int, maxRet int, errRet error) {
	resp, err := http.Get(fmt.Sprintf("http://%s:%d%s", eg.host, eg.port, GetMinMaxPath))
	if err != nil {
		return -1, -1, fmt.Errorf("http get error: %v", err)
	}
	defer resp.Body.Close()
	bodyContents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -1, -1, fmt.Errorf("error reading response body %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return -1, -1, fmt.Errorf("error %d reading GetMinMax response", resp.StatusCode)
	}
	minMaxResponse := MinMaxResponse{}
	err = json.Unmarshal(bodyContents, &minMaxResponse)
	if err != nil {
		return -1, -1, fmt.Errorf("error unmarshalling response %w", err)
	}
	if minMaxResponse.MinAvail == 0 {
		return -1, -1, fmt.Errorf("invalid decoded json: %+v", minMaxResponse)
	}

	return minMaxResponse.MinAvail, minMaxResponse.MaxSeen, nil
}

// Return the event with the provided sequence number.  Returns the appropriate
// EventInfo or an error if no such event is known, or the event has already
// been trimmed.
func (eg *eventGetter) getEntry(seqno int) (EventInfo, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s:%d%s/%d", eg.host, eg.port, GetEntryPath, seqno))
	if err != nil {
		return EventInfo{}, fmt.Errorf("http get err %v", err)
	}
	defer resp.Body.Close()
	bodyContents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return EventInfo{}, fmt.Errorf("error reading response body %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return EventInfo{}, fmt.Errorf("error %d reading GetEntry response", resp.StatusCode)
	}
	entryResponse := EventInfo{}
	err = json.Unmarshal(bodyContents, &entryResponse)
	if err != nil {
		return EventInfo{}, fmt.Errorf("error unmarshalling response %w", err)
	}
	if len(entryResponse.Error) == 0 && entryResponse.Event == nil {
		return EventInfo{}, fmt.Errorf("invalid decoded json: %+v", entryResponse)
	}

	return entryResponse, nil
}

// Trim the events up to and including seqno from the recordevents pod.
// Returns an error if a nonsensical seqno is passed in, but does not return
// error for trimming already trimmed regions.
func (eg *eventGetter) trimThrough(seqno int) error {
	resp, err := http.Post(fmt.Sprintf("http://%s:%d%s/%d", eg.host, eg.port, TrimThroughPath, seqno), "", nil)
	if err != nil {
		return fmt.Errorf("http post err %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error %d reading TrimThrough response: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Clean up the getter by tearing down the port forward.
func (eg *eventGetter) cleanup() {
	pid := eg.forwardPID
	eg.forwardPID = 0
	if pid != 0 {
		monitoring.Cleanup(pid)
	}
}

// Wait (up to timeoutEvRetry) for the pod to RestAPI to answer request.
func (eg *eventGetter) waitTillUp() error {
	var internalErr error
	wait.PollImmediate(minEvRetryInterval, timeoutEvRetry, func() (bool, error) {
		_, _, internalErr = eg.getMinMax()
		if internalErr != nil {
			return false, nil
		}
		return true, nil
	})
	if internalErr != nil {
		return fmt.Errorf("timeout waiting for recordevents pod to come up: %v", internalErr)
	}
	return nil
}
