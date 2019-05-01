// Copyright 2019, OpenCensus Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stackdriver // import "contrib.go.opencensus.io/exporter/stackdriver"

import (
	"contrib.go.opencensus.io/resource/resourcekeys"
	"go.opencensus.io/resource"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

type resourceMap struct {
	// Mapping from the input resource type to the monitored resource type in Stackdriver.
	srcType, dstType string
	// Mapping from Stackdriver monitored resource label to an OpenCensus resource label.
	labels map[string]string
}

// Resource labels that are generally internal to the exporter.
// Consider exposing these labels and a type identifier in the future to allow
// for customization.
const (
	stackdriverLocation             = "contrib.opencensus.io/exporter/stackdriver/location"
	stackdriverProjectID            = "contrib.opencensus.io/exporter/stackdriver/project_id"
	stackdriverGenericTaskNamespace = "contrib.opencensus.io/exporter/stackdriver/generic_task/namespace"
	stackdriverGenericTaskJob       = "contrib.opencensus.io/exporter/stackdriver/generic_task/job"
	stackdriverGenericTaskID        = "contrib.opencensus.io/exporter/stackdriver/generic_task/task_id"
)

// Mappings for the well-known OpenCensus resources to applicable Stackdriver resources.
var resourceMappings = []resourceMap{
	{
		srcType: resourcekeys.K8STypeContainer,
		dstType: "k8s_container",
		labels: map[string]string{
			"project_id":     stackdriverProjectID,
			"location":       stackdriverLocation,
			"cluster_name":   resourcekeys.K8SKeyClusterName,
			"namespace_name": resourcekeys.K8SKeyNamespaceName,
			"pod_name":       resourcekeys.K8SKeyPodName,
			"container_name": resourcekeys.K8SKeyContainerName,
		},
	},
	{
		srcType: resourcekeys.GCPTypeGCEInstance,
		dstType: "gce_instance",
		labels: map[string]string{
			"project_id":  resourcekeys.GCPKeyGCEProjectID,
			"instance_id": resourcekeys.GCPKeyGCEInstanceID,
			"zone":        resourcekeys.GCPKeyGCEZone,
		},
	},
	{
		srcType: resourcekeys.AWSTypeEC2Instance,
		dstType: "aws_ec2_instance",
		labels: map[string]string{
			"project_id":  stackdriverProjectID,
			"instance_id": resourcekeys.AWSKeyEC2InstanceID,
			"region":      resourcekeys.AWSKeyEC2Region,
			"aws_account": resourcekeys.AWSKeyEC2AccountID,
		},
	},
	// Fallback to generic task resource.
	{
		srcType: "",
		dstType: "generic_task",
		labels: map[string]string{
			"project_id": stackdriverProjectID,
			"location":   stackdriverLocation,
			"namespace":  stackdriverGenericTaskNamespace,
			"job":        stackdriverGenericTaskJob,
			"task_id":    stackdriverGenericTaskID,
		},
	},
}

func DefaultMapResource(res *resource.Resource) *monitoredrespb.MonitoredResource {
Outer:
	for _, rm := range resourceMappings {
		if res.Type != rm.srcType {
			continue
		}
		result := &monitoredrespb.MonitoredResource{
			Type:   rm.dstType,
			Labels: make(map[string]string, len(rm.labels)),
		}
		for dst, src := range rm.labels {
			if v, ok := res.Labels[src]; ok {
				result.Labels[dst] = v
			} else {
				// A required label wasn't filled at all. Try subsequent mappings.
				continue Outer
			}
		}
		return result
	}
	return nil
}
