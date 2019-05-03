// Copyright 2018, OpenCensus Authors
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

// Package resourcekeys contains well known type and label keys for resources.
package resourcekeys // import "contrib.go.opencensus.io/resource/resourcekeys"

// Constants for Kubernetes resources.
const (
	K8STypeContainer = "k8s.io/container"

	// A uniquely identifying name for the Kubernetes cluster. Kubernetes
	// does not have cluster names as an internal concept so this may be
	// set to any meaningful value within the environment. For example,
	// GKE clusters have a name which can be used for this label.
	K8SKeyClusterName   = "k8s.io/cluster/name"
	K8SKeyNamespaceName = "k8s.io/namespace/name"
	K8SKeyPodName       = "k8s.io/pod/name"
	K8SKeyContainerName = "k8s.io/container/name"
)

// Constants for AWS resources.
const (
	AWSTypeEC2Instance = "aws.com/ec2/instance"

	AWSKeyEC2AccountID  = "aws.com/ec2/account_id"
	AWSKeyEC2Region     = "aws.com/ec2/region"
	AWSKeyEC2InstanceID = "aws.com/ec2/instance_id"
)

// Constants for GCP resources.
const (
	GCPTypeGCEInstance = "cloud.google.com/gce/instance"

	// ProjectID of the GCE VM. This is not the project ID of the used client credentials.
	GCPKeyGCEProjectID  = "cloud.google.com/gce/project_id"
	GCPKeyGCEZone       = "cloud.google.com/gce/zone"
	GCPKeyGCEInstanceID = "cloud.google.com/gce/instance_id"
)
