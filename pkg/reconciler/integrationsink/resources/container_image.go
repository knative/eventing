package resources

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/pkg/kmeta"
)

//func MakePod(sink *v1alpha1.IntegrationSink) *corev1.Pod {
//
//	pod := &corev1.Pod{
//		TypeMeta: metav1.TypeMeta{
//			APIVersion: "v1",
//			Kind:       "Pod",
//		},
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      DeploymentName(sink),
//			Namespace: sink.Namespace,
//			OwnerReferences: []metav1.OwnerReference{
//				*kmeta.NewControllerRef(sink),
//			},
//			Labels: map[string]string{
//				"app": DeploymentName(sink),
//			},
//		},
//		Spec: corev1.PodSpec{
//			Containers: []corev1.Container{
//				{
//					Name:            "sink",
//					Image:           "quay.io/openshift-knative/kn-connector-log-sink:1.0-SNAPSHOT",
//					ImagePullPolicy: corev1.PullIfNotPresent,
//					Ports: []corev1.ContainerPort{{
//						ContainerPort: 8080,
//						Protocol:      corev1.ProtocolTCP,
//						Name:          "http",
//					}},
//				},
//			},
//		},
//	}
//	return pod
//}

func MakeDeploymentSpec(sink *v1alpha1.IntegrationSink) *appsv1.Deployment {

	//labels := Labels(sink.Name)
	//for k, v := range labels {
	//	template.Labels[k] = v
	//}

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName(sink),
			Namespace: sink.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(sink),
			},
			Labels: Labels(sink.Name),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: Labels(sink.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: Labels(sink.Name),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "sink",
							Image:           "quay.io/openshift-knative/kn-connector-log-sink:1.0-SNAPSHOT",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{{
								ContainerPort: 8080,
								Protocol:      corev1.ProtocolTCP,
								Name:          "http",
							}},
						},
					},
				},
			},
		},
	}

	return deploy

}

func MakeService(sink *v1alpha1.IntegrationSink) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName(sink),
			Namespace: sink.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(sink),
			},
			Labels: Labels(sink.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: Labels(sink.Name),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.IntOrString{IntVal: 8080},
				},
			},
		},
	}
}

func MakeDeployment(sink *v1alpha1.IntegrationSink) *appsv1.Deployment {

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName(sink),
			Namespace: sink.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(sink),
			},
			Labels: map[string]string{
				"app": DeploymentName(sink),
			},
		},
		//Spec: appsv1.DeploymentSpec{
		//	Selector: &metav1.LabelSelector{
		//		MatchLabels: labels,
		//	},
		//	Template: template,
		//},
	}
	return deploy

}

func DeploymentName(sink *v1alpha1.IntegrationSink) string {
	return kmeta.ChildName(sink.Name, "-deployment")
}

//func Labels(name string) map[string]string {
//	return map[string]string{
//		"sources.knative.dev/source":          containerSourceController,
//		"sources.knative.dev/containerSource": name,
//	}
//}
