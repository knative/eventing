/*
 * Copyright 2018 the original author or authors.
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

package v1alpha2

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// RouteRule HACK
type RouteRule struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               RouteRuleSpec    `json:"spec"`
	Status             *RouteRuleStatus `json:"status,omitempty"`
}

// RouteRuleSpec HACK
type RouteRuleSpec struct {
	Destination IstioService        `json:"destination"`
	Route       []DestinationWeight `json:"route"`
	Rewrite     HTTPRewrite         `json:"rewrite"`
}

// IstioService HACK
type IstioService struct {
	Domain    string `json:"domain"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// DestinationWeight HACK
type DestinationWeight struct {
	Destination IstioService `json:"destination"`
	Weight      int32        `json:"weight"`
}

// HTTPRewrite HACK
type HTTPRewrite struct {
	Authority string `json:"authority"`
}

// RouteRuleStatus HACK
type RouteRuleStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HACK
type RouteRuleList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []RouteRule `json:"items"`
}
