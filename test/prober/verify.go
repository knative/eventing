/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package prober

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/lib"
	"math/rand"
	"net/http"
	"sort"
	"strings"
)

func (p *prober) Verify() ([]error, int) {
	hostname := p.figureOutClusterHostname()
	report := p.fetchReceiverReport(hostname)
	p.log.Infof("Fetched receiver report. Events propagated: %v. "+
		"State: %v", report.Events, report.State)
	if report.State == "active" {
		panic(errors.New("report fetched to early, receiver is in active state"))
	}
	errs := make([]error, 0)
	for _, t := range report.Thrown {
		errs = append(errs, errors.New(t))
	}
	return errs, report.Events
}

func (p *prober) Finish() {
	p.removeSender()
}

type nodeAddresses []*corev1.NodeAddress

type byAddressType struct {
	nodeAddresses
}

func (s byAddressType) Len() int {
	return len(s.nodeAddresses)
}

func (s byAddressType) Swap(i, j int) {
	s.nodeAddresses[i], s.nodeAddresses[j] = s.nodeAddresses[j], s.nodeAddresses[i]
}

func (s byAddressType) Less(i, j int) bool {
	return addressTypeToPriority(s.nodeAddresses[i]) < addressTypeToPriority(s.nodeAddresses[j])
}

var addressTypeToPriorities = map[corev1.NodeAddressType]int{
	corev1.NodeExternalDNS: 0,
	corev1.NodeExternalIP:  1,
	corev1.NodeInternalIP:  2,
	corev1.NodeInternalDNS: 3,
	corev1.NodeHostName:    4,
}

func (p *prober) figureOutClusterHostname() string {
	nodes, err := p.client.Kube.Kube.CoreV1().Nodes().List(metav1.ListOptions{})
	lib.NoError(err)
	if len(nodes.Items) == 1 {
		node := nodes.Items[0]
		return p.nodeExternalAddress(node)
	} else {
		workers := p.filterOutMasters(nodes.Items)
		worker := workers[rand.Intn(len(workers))]
		return p.nodeExternalAddress(worker)
	}
}

func (p *prober) nodeExternalAddress(node corev1.Node) string {
	return p.prioritizeAddresses(node.Status.Addresses)[0].Address
}

func (p *prober) prioritizeAddresses(addresses []corev1.NodeAddress) []*corev1.NodeAddress {
	result := make([]*corev1.NodeAddress, 0)
	for i := 0; i < len(addresses); i++ {
		result = append(result, &addresses[i])
	}
	sort.Sort(byAddressType{result})
	return result
}

func addressTypeToPriority(address *corev1.NodeAddress) int {
	return addressTypeToPriorities[address.Type]
}

func (p *prober) filterOutMasters(nodes []corev1.Node) []corev1.Node {
	result := make([]corev1.Node, 0)
	for _, node := range nodes {
		if node.Labels == nil {
			result = append(result, node)
			continue
		}
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; !ok {
			result = append(result, node)
		}
	}
	return result
}

func (p *prober) fetchReceiverReport(hostname string) *Report {
	u := fmt.Sprintf("http://%s:%d/report", hostname, receiverNodePort)
	p.log.Infof("Fetching receiver report at: %v", u)
	resp, err := http.Get(u)
	lib.NoError(err)
	if resp.StatusCode != 200 {
		var b strings.Builder
		lib.NoError(resp.Header.Write(&b))
		headers := b.String()
		panic(fmt.Errorf("could not get receiver report at %v, "+
			"status code: %v, headers: %v", u, resp.StatusCode, headers))
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	lib.NoError(err)
	lib.NoError(resp.Body.Close())
	jsonBytes := buf.Bytes()
	var report Report
	lib.NoError(json.Unmarshal(jsonBytes, &report))
	return &report
}

// Report represents a receiver JSON report
type Report struct {
	State  string   `json:"state"`
	Events int      `json:"events"`
	Thrown []string `json:"thrown"`
}
