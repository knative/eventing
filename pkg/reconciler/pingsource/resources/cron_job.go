package resources

import (
	"encoding/json"
	v1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/kmeta"
)

// MakeReceiveAdapter generates (but does not insert into K8s) the Receive Adapter Deployment for
// PingSources.
func MakeCronJob(args *Args) *batchv1beta1.CronJob {
	RequestResourceCPU, err := resource.ParseQuantity(args.Source.Spec.Resources.Requests.ResourceCPU)
	if err != nil {
		RequestResourceCPU = resource.MustParse("100m")
	}
	RequestResourceMemory, err := resource.ParseQuantity(args.Source.Spec.Resources.Requests.ResourceMemory)
	if err != nil {
		RequestResourceMemory = resource.MustParse("100Mi")
	}
	LimitResourceCPU, err := resource.ParseQuantity(args.Source.Spec.Resources.Limits.ResourceCPU)
	if err != nil {
		LimitResourceCPU = resource.MustParse("100m")
	}
	LimitResourceMemory, err := resource.ParseQuantity(args.Source.Spec.Resources.Limits.ResourceMemory)
	if err != nil {
		LimitResourceMemory = resource.MustParse("100Mi")
	}

	res := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    RequestResourceCPU,
			corev1.ResourceMemory: RequestResourceMemory,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    LimitResourceCPU,
			corev1.ResourceMemory: LimitResourceMemory,
		},
	}

	ceOverrides, err := json.Marshal(args.Source.Spec.SourceSpec.CloudEventOverrides)
	if err != nil {
		ceOverrides = []byte("")
	}

	podSpec := corev1.PodSpec{
		ServiceAccountName: args.Source.Spec.ServiceAccountName,
		RestartPolicy:      "Never",
		Containers: []corev1.Container{{
			Name:  "ping",
			Image: args.Image,
			Env: []corev1.EnvVar{{
				Name:  "DATA",
				Value: args.Source.Spec.Data,
			}, {
				Name:  "K_CE_OVERRIDES", // TODO this should come from SinkBinding
				Value: string(ceOverrides),
			}, {
				Name:  "NAME",
				Value: args.Source.Name,
			}, {
				Name:  "NAMESPACE",
				Value: args.Source.Namespace,
			}, {
				Name:  "METRICS_DOMAIN", // TODO: not sure this will work the way it use to.
				Value: "knative.dev/eventing",
			}, {
				Name:  "K_METRICS_CONFIG",
				Value: args.MetricsConfig,
			}, {
				Name:  "K_LOGGING_CONFIG",
				Value: args.LoggingConfig,
			}},
			Resources: res,
		}},
	}

	cj := &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Source),
			},
			Name:      kmeta.ChildName(args.Source.Name, "-"+"pingsource"),
			Namespace: args.Source.Namespace,
			Labels:    Labels(args.Source.Name),
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule:          args.Source.Spec.Schedule,
			ConcurrencyPolicy: "Forbid",
			JobTemplate: batchv1beta1.JobTemplateSpec{
				Spec: v1.JobSpec{
					BackoffLimit: pointer.Int32Ptr(0),
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			},
		},
	}
	return cj
}
