package eventtype

import (
	"github.com/google/go-cmp/cmp"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

const (
	feedName = "feed-name"
)

type mapTestCase struct {
	name string
	obj  handler.MapObject
	rr   []reconcile.Request
}

func TestMap(t *testing.T) {
	testCases := []mapTestCase{
		{
			name: "not a Feed",
			obj: handler.MapObject{
				Object: getPod(),
			},
			rr: []reconcile.Request{},
		},
		{
			name: "Feed",
			obj: handler.MapObject{
				Meta:   getFeed(etNamespace, feedName, etName),
				Object: getFeed(etNamespace, feedName, etName),
			},
			rr: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: etNamespace,
						Name:      etName,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			feedToEventType := feedToEventType{}
			actualRR := feedToEventType.Map(tc.obj)
			if diff := cmp.Diff(tc.rr, actualRR); diff != "" {
				t.Errorf("Reconcile request (-want, +got) = %v", diff)
			}
		})
	}
}

func getPod() *v1.Pod {
	return &v1.Pod{
		ObjectMeta: objectMeta(etNamespace, "pod-name"),
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       "Pod",
		},
	}
}

