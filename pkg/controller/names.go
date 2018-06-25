/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package controller

import "fmt"

func BusProvisionerDeploymentName(busName string) string {
	return fmt.Sprintf("%s-bus-provisioner", busName)
}

func BusDispatcherDeploymentName(busName string) string {
	return fmt.Sprintf("%s-bus", busName)
}

func BusServiceAccountName(busName string) string {
	return fmt.Sprintf("%s-bus", busName)
}

func BusClusterRoleBindingName(busName string) string {
	return fmt.Sprintf("%s-bus", busName)
}

func BusDispatcherServiceName(busName string) string {
	return fmt.Sprintf("%s-bus", busName)
}

func ChannelVirtualServiceName(channelName string) string {
	return fmt.Sprintf("%s-channel", channelName)
}

func ChannelServiceName(channelName string) string {
	return fmt.Sprintf("%s-channel", channelName)
}

func ChannelHostName(channelName, namespace string) string {
	return fmt.Sprintf("%s.%s.channels.cluster.local", channelName, namespace)
}

func ServiceHostName(serviceName, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)
}
