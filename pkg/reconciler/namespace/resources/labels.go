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

package resources

const (
	// Label to enable knative-eventing in a namespace.
	InjectionLabelKey           = "knative-eventing-injection"
	InjectionEnabledLabelValue  = "enabled"
	InjectionDisabledLabelValue = "disabled"
	InjectedResourceLabel       = "eventing.knative.dev/namespaceInjected"
	CmpDefaultLabelKey          = "knative.dev/config-category"
	CmpDefaultLabelValue        = "eventing"
)

// OwnedLabels generates the labels present on injected broker resources.
func OwnedLabels() map[string]string {
	return map[string]string{
		InjectedResourceLabel: "true",
	}
}

// ConfigMapPropagationOwnedLabels generates the labels present on injected broker resources.
func ConfigMapPropagationOwnedLabels() map[string]string {
	return map[string]string{
		CmpDefaultLabelKey: CmpDefaultLabelValue,
	}
}

func InjectionEnabledLabels() map[string]string {
	return map[string]string{
		InjectionLabelKey: InjectionEnabledLabelValue,
	}
}

func InjectionDisabledLabels() map[string]string {
	return map[string]string{
		InjectionLabelKey: InjectionDisabledLabelValue,
	}
}
