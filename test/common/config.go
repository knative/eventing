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

package common

// ValidProvisionersMap saves the provisioner-features mapping.
// Each pair means the provisioner support the list of features.
var ValidProvisionersMap = map[string][]Feature{
	InMemoryProvisioner:  {FeatureBasic},
	GCPPubSubProvisioner: {FeatureBasic, FeatureRedelivery, FeaturePersistence},
	KafkaProvisioner:     {FeatureBasic, FeatureRedelivery, FeaturePersistence},
	NatssProvisioner:     {FeatureBasic, FeatureRedelivery, FeaturePersistence},
}

// Feature is the feature supported by the Channel provisioner.
type Feature string

const (
	// FeatureBasic is the feature that should be supported by all Channel provisioners
	FeatureBasic Feature = "basic"
	// FeatureRedelivery means if downstream rejects an event, that request will be attempted again.
	FeatureRedelivery Feature = "redelivery"
	// FeaturePersistence means if Channel's Pod goes down, all events already ACKed by the Channel
	// will persist and be retransmitted when the Pod restarts.
	FeaturePersistence Feature = "persistence"
)
