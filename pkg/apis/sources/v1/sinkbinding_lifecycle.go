/*
Copyright 2020 The Knative Authors

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

package v1

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
)

const (
	oidcTokenVolumeName = "oidc-token"
)

var sbCondSet = apis.NewLivingConditionSet(
	SinkBindingConditionSinkProvided,
	SinkBindingConditionOIDCIdentityCreated,
	SinkBindingConditionOIDCTokenSecretCreated,
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*SinkBinding) GetConditionSet() apis.ConditionSet {
	return sbCondSet
}

// GetGroupVersionKind returns the GroupVersionKind.
func (*SinkBinding) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("SinkBinding")
}

// GetUntypedSpec implements apis.HasSpec
func (s *SinkBinding) GetUntypedSpec() interface{} {
	return s.Spec
}

// GetSubject implements psbinding.Bindable
func (sb *SinkBinding) GetSubject() tracker.Reference {
	return sb.Spec.Subject
}

// GetBindingStatus implements psbinding.Bindable
func (sb *SinkBinding) GetBindingStatus() duck.BindableStatus {
	return &sb.Status
}

// SetObservedGeneration implements psbinding.BindableStatus
func (sbs *SinkBindingStatus) SetObservedGeneration(gen int64) {
	sbs.ObservedGeneration = gen
}

// InitializeConditions populates the SinkBindingStatus's conditions field
// with all of its conditions configured to Unknown.
func (sbs *SinkBindingStatus) InitializeConditions() {
	sbCondSet.Manage(sbs).InitializeConditions()
}

// MarkBindingUnavailable marks the SinkBinding's Ready condition to False with
// the provided reason and message.
func (sbs *SinkBindingStatus) MarkBindingUnavailable(reason, message string) {
	sbCondSet.Manage(sbs).MarkFalse(SinkBindingConditionReady, reason, message)
}

// MarkBindingAvailable marks the SinkBinding's Ready condition to True.
func (sbs *SinkBindingStatus) MarkBindingAvailable() {
	sbCondSet.Manage(sbs).MarkTrue(SinkBindingConditionReady)
}

// MarkSink sets the condition that the source has a sink configured.
func (sbs *SinkBindingStatus) MarkSink(addr *duckv1.Addressable) {
	if addr != nil {
		sbs.SinkURI = addr.URL
		sbs.SinkCACerts = addr.CACerts
		sbs.SinkAudience = addr.Audience
		sbCondSet.Manage(sbs).MarkTrue(SinkBindingConditionSinkProvided)
	} else {
		sbCondSet.Manage(sbs).MarkFalse(SinkBindingConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

func (sbs *SinkBindingStatus) MarkOIDCIdentityCreatedSucceeded() {
	sbCondSet.Manage(sbs).MarkTrue(SinkBindingConditionOIDCIdentityCreated)
}

func (sbs *SinkBindingStatus) MarkOIDCIdentityCreatedSucceededWithReason(reason, messageFormat string, messageA ...interface{}) {
	sbCondSet.Manage(sbs).MarkTrueWithReason(SinkBindingConditionOIDCIdentityCreated, reason, messageFormat, messageA...)
}

func (sbs *SinkBindingStatus) MarkOIDCIdentityCreatedFailed(reason, messageFormat string, messageA ...interface{}) {
	sbCondSet.Manage(sbs).MarkFalse(SinkBindingConditionOIDCIdentityCreated, reason, messageFormat, messageA...)
}

func (sbs *SinkBindingStatus) MarkOIDCIdentityCreatedUnknown(reason, messageFormat string, messageA ...interface{}) {
	sbCondSet.Manage(sbs).MarkUnknown(SinkBindingConditionOIDCIdentityCreated, reason, messageFormat, messageA...)
}

func (sbs *SinkBindingStatus) MarkOIDCTokenSecretCreatedSuccceeded() {
	sbCondSet.Manage(sbs).MarkTrue(SinkBindingConditionOIDCTokenSecretCreated)
}

func (sbs *SinkBindingStatus) MarkOIDCTokenSecretCreatedSuccceededWithReason(reason, messageFormat string, messageA ...interface{}) {
	sbCondSet.Manage(sbs).MarkTrueWithReason(SinkBindingConditionOIDCTokenSecretCreated, reason, messageFormat, messageA...)
}

func (sbs *SinkBindingStatus) MarkOIDCTokenSecretCreatedFailed(reason, messageFormat string, messageA ...interface{}) {
	sbCondSet.Manage(sbs).MarkFalse(SinkBindingConditionOIDCTokenSecretCreated, reason, messageFormat, messageA...)
}

func (sbs *SinkBindingStatus) MarkOIDCTokenSecretCreatedUnknown(reason, messageFormat string, messageA ...interface{}) {
	sbCondSet.Manage(sbs).MarkUnknown(SinkBindingConditionOIDCTokenSecretCreated, reason, messageFormat, messageA...)
}

// Do implements psbinding.Bindable
func (sb *SinkBinding) Do(ctx context.Context, ps *duckv1.WithPod) {
	// First undo so that we can just unconditionally append below.
	sb.Undo(ctx, ps)

	resolver := GetURIResolver(ctx)
	if resolver == nil {
		logging.FromContext(ctx).Errorf("No Resolver associated with context for sink: %+v", sb)
		return
	}
	addr, err := resolver.AddressableFromDestinationV1(ctx, sb.Spec.Sink, sb)
	if err != nil {
		logging.FromContext(ctx).Errorw("URI could not be extracted from destination: ", zap.Error(err))
		return
	}
	sb.Status.MarkSink(addr)

	var ceOverrides string
	if sb.Spec.CloudEventOverrides != nil {
		if co, err := json.Marshal(sb.Spec.SourceSpec.CloudEventOverrides); err != nil {
			logging.FromContext(ctx).Errorw(fmt.Sprintf("Failed to marshal CloudEventOverrides into JSON for %+v", sb), zap.Error(err))
		} else if len(co) > 0 {
			ceOverrides = string(co)
		}
	}

	spec := ps.Spec.Template.Spec
	for i := range spec.InitContainers {
		spec.InitContainers[i].Env = append(spec.InitContainers[i].Env, corev1.EnvVar{
			Name:  "K_SINK",
			Value: addr.URL.String(),
		})
		if addr.CACerts != nil {
			spec.InitContainers[i].Env = append(spec.InitContainers[i].Env, corev1.EnvVar{
				Name:  "K_CA_CERTS",
				Value: *addr.CACerts,
			})
		}
		spec.InitContainers[i].Env = append(spec.InitContainers[i].Env, corev1.EnvVar{
			Name:  "K_CE_OVERRIDES",
			Value: ceOverrides,
		})
	}
	for i := range spec.Containers {
		spec.Containers[i].Env = append(spec.Containers[i].Env, corev1.EnvVar{
			Name:  "K_SINK",
			Value: addr.URL.String(),
		})
		if addr.CACerts != nil {
			spec.Containers[i].Env = append(spec.Containers[i].Env, corev1.EnvVar{
				Name:  "K_CA_CERTS",
				Value: *addr.CACerts,
			})
		}
		spec.Containers[i].Env = append(spec.Containers[i].Env, corev1.EnvVar{
			Name:  "K_CE_OVERRIDES",
			Value: ceOverrides,
		})
	}

	if sb.Status.OIDCTokenSecretName != nil {
		ps.Spec.Template.Spec.Volumes = append(ps.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: oidcTokenVolumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: *sb.Status.OIDCTokenSecretName,
								},
							},
						},
					},
				},
			},
		})

		for i := range spec.Containers {
			spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts, corev1.VolumeMount{
				Name:      oidcTokenVolumeName,
				MountPath: "/oidc",
			})
		}
		for i := range spec.InitContainers {
			spec.InitContainers[i].VolumeMounts = append(spec.InitContainers[i].VolumeMounts, corev1.VolumeMount{
				Name:      oidcTokenVolumeName,
				MountPath: "/oidc",
			})
		}
	}
}

func (sb *SinkBinding) Undo(ctx context.Context, ps *duckv1.WithPod) {
	spec := ps.Spec.Template.Spec
	for i, c := range spec.InitContainers {
		if len(c.Env) == 0 {
			continue
		}
		env := make([]corev1.EnvVar, 0, len(spec.InitContainers[i].Env))
		for j, ev := range c.Env {
			switch ev.Name {
			case "K_SINK", "K_CE_OVERRIDES", "K_CA_CERTS":
				continue
			default:
				env = append(env, spec.InitContainers[i].Env[j])
			}
		}
		spec.InitContainers[i].Env = env

		if len(spec.InitContainers[i].VolumeMounts) > 0 {
			volumeMounts := make([]corev1.VolumeMount, 0, len(spec.InitContainers[i].VolumeMounts))
			for j, vol := range c.VolumeMounts {
				if vol.Name == oidcTokenVolumeName {
					continue
				}
				volumeMounts = append(volumeMounts, spec.InitContainers[i].VolumeMounts[j])
			}
			spec.InitContainers[i].VolumeMounts = volumeMounts
		}
	}
	for i, c := range spec.Containers {
		if len(c.Env) == 0 {
			continue
		}
		env := make([]corev1.EnvVar, 0, len(spec.Containers[i].Env))
		for j, ev := range c.Env {
			switch ev.Name {
			case "K_SINK", "K_CE_OVERRIDES", "K_CA_CERTS":
				continue
			default:
				env = append(env, spec.Containers[i].Env[j])
			}
		}
		spec.Containers[i].Env = env

		if len(spec.Containers[i].VolumeMounts) > 0 {
			volumeMounts := make([]corev1.VolumeMount, 0, len(spec.Containers[i].VolumeMounts))
			for j, vol := range c.VolumeMounts {
				if vol.Name == oidcTokenVolumeName {
					continue
				}
				volumeMounts = append(volumeMounts, spec.Containers[i].VolumeMounts[j])
			}
			spec.Containers[i].VolumeMounts = volumeMounts
		}
	}

	if len(spec.Volumes) > 0 {
		volumes := make([]corev1.Volume, 0, len(spec.Volumes))
		for i, vol := range spec.Volumes {
			if vol.Name == oidcTokenVolumeName {
				continue
			}
			volumes = append(volumes, spec.Volumes[i])
		}
		ps.Spec.Template.Spec.Volumes = volumes
	}
}
