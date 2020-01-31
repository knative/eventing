package resources

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/tracker"
)

var subjectGVK = batchv1.SchemeGroupVersion.WithKind("Job")

// MakeReceiveAdapter generates (but does not insert into K8s) the Receive Adapter Deployment for
// PingSources.
func MakeSinkBinding(args *Args) *v1alpha1.SinkBinding {
	subjectAPIVersion, subjectKind := subjectGVK.ToAPIVersionAndKind()

	sb := &v1alpha1.SinkBinding{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Source),
			},
			Name:      kmeta.ChildName(args.Source.Name, "-"+"pingsource"),
			Namespace: args.Source.Namespace,
		},
		Spec: v1alpha1.SinkBindingSpec{
			SourceSpec: args.Source.Spec.SourceSpec,
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: subjectAPIVersion,
					Kind:       subjectKind,
					Selector: &metav1.LabelSelector{
						MatchLabels: Labels(args.Source.Name),
					},
				},
			},
		},
	}
	return sb
}
