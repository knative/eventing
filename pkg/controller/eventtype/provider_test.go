package eventtype

import (
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
			if !rrEquals(tc.rr, actualRR) {
				t.Errorf("Expected reconcile requests %v, actual reconcile requests %v", tc.rr, actualRR)
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

// rrEquals determines if the expected and actual slices have the same reconcile requests.
func rrEquals(e, a []reconcile.Request) bool {
	if e == nil {
		if a == nil {
			return true
		}
		return false
	} else if a == nil {
		return false
	} else if len(e) != len(a) {
		return false
	} else {
		for i := range e {
			if e[i] != a[i] {
				return false
			}
		}
	}
	return true
}
