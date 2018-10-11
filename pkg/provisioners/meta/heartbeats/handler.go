package heartbeats

import (
	"fmt"
	"reflect"
)

/*

Depended on

apiVersion: metacontroller.k8s.io/v1alpha1
kind: CompositeController
metadata:
  name: heartbeats-controller
spec:
  generateSelector: true
  parentResource:
    apiVersion: eventing.knative.dev/v1alpha1
    resource: sources
  childResources:
  - apiVersion: eventing.knative.dev/v1alpha1
    resource: channels
  - apiVersion: v1
    resource: pods
  hooks:
    sync:
      webhook:
        url: http://heartbeats-provisioner.default/
        timeout: 30s


*/

const (
	//image = "github.com/n3wscott/metacontroller-go-sdk/cmd/heartbeats"
	image = "gcr.io/plori-nicholss/heartbeats-83786fa49704c68848513f3f5ee000df@sha256:9be8ba27e2daa4df91fdfe3006a1a99c022663a031d597adbe019b45fca28c77"
)

type HeartBeatArguments struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Period int    `json:"period"`
}

type handler struct{}

func NewHandler() sdk.Handler {
	return &handler{}
}

func (h *handler) Convert(ctx context.Context, versionKind string, body []byte) (runtime.Object, error) {
	log.Printf("Converting %s", versionKind)
	if versionKind == "Channel.eventing.knative.dev/v1alpha1" {
		channel := &eventingv1alpha1.Channel{}
		if err := json.Unmarshal(body, channel); err != nil {
			return nil, err
		}
		return channel, nil
	}
	if versionKind == "Pod.v1" {
		pod := &corev1.Pod{}
		if err := json.Unmarshal(body, pod); err != nil {
			return nil, err
		}
		return pod, nil
	}
	return nil, fmt.Errorf("unknown verison kind: %s", versionKind)
}

func (h *handler) Handle(ctx context.Context, source eventingv1alpha1.Source, c []runtime.Object) (*eventingv1alpha1.SourceStatus, []runtime.Object, error) {

	log.Printf("Starting Handle for: %s", source.Name)

	args := &HeartBeatArguments{}

	if source.Spec.Arguments != nil {
		if err := json.Unmarshal(source.Spec.Arguments.Raw, args); err != nil {
			log.Printf("Error: %s failed to unmarshal arguments, %v", source.Name, err)
		}
	}
	args.Name = source.Name

	var orgChannel *eventingv1alpha1.Channel
	var orgPod *corev1.Pod
	//c[0].GetObjectKind().GroupVersionKind().Kind <-- todo could do it this way.
	for _, obj := range c {
		log.Printf("obj is a kind %s", reflect.TypeOf(obj))
		if orgObj, ok := obj.(*eventingv1alpha1.Channel); ok {
			orgChannel = orgObj
		}
		if orgObj, ok := obj.(*corev1.Pod); ok {
			orgPod = orgObj
		}
	}

	channel := makeChannel(orgChannel, args)
	pod := makePod(orgPod, orgChannel, args)

	status := updateStatus(source.Status, orgChannel, channel, orgPod, pod)

	var children []runtime.Object
	if channel != nil {
		children = append(children, channel)
	}
	if pod != nil {
		children = append(children, pod)
	}

	return status, children, nil
}

func updateStatus(org eventingv1alpha1.SourceStatus, orgChan, channel *eventingv1alpha1.Channel, orgPod, pod *corev1.Pod) *eventingv1alpha1.SourceStatus {
	status := org.DeepCopy()

	if orgChan != nil && orgPod != nil {
		c := orgChan.Status.GetCondition(eventingv1alpha1.ChannelConditionReady)
		if c.IsTrue() && orgPod != nil { // TODO:: could look at pod status.
			status.MarkProvisioned()
			return status
		}
	}

	cM := ""
	pM := ""
	if orgChan == nil {
		if channel == nil {
			cM = "Channel nil"
		} else {
			cM = "Channel created"
		}
	} else {
		c := orgChan.Status.GetCondition(eventingv1alpha1.ChannelConditionReady)
		cM = fmt.Sprintf("Channel.isReady: %v", c.IsTrue())
	}

	if orgPod == nil {
		if pod == nil {
			pM = "Pod nil"
		} else {
			pM = "Pod created"
		}
	} else {
		pM = fmt.Sprintf("Pod.isReady: %v", orgPod != nil)
	}
	status.MarkDeprovisioned("Provisioning", "%s; %s", cM, pM)
	return status
}

func makeChannel(org *eventingv1alpha1.Channel, args *HeartBeatArguments) *eventingv1alpha1.Channel {
	channel := &eventingv1alpha1.Channel{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Channel",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: args.Name + "-chan",
		},
		Spec: eventingv1alpha1.ChannelSpec{
			Provisioner: &eventingv1alpha1.ProvisionerReference{
				Ref: &corev1.ObjectReference{
					Name:       "in-memory-bus-provisioner",
					APIVersion: "eventing.knative.dev/v1alpha1",
					Kind:       "ClusterProvisioner",
				},
			},
			Channelable: &duckv1alpha1.Channelable{
				Subscribers: []duckv1alpha1.ChannelSubscriberSpec{{
					SinkableDomain: "message-dumper.default.svc.cluster.local",
				}},
			},
		},
	}
	if org != nil {
		channel.Spec.Generation = org.Spec.Generation
	}
	return channel
}

func makePod(org *corev1.Pod, channel *eventingv1alpha1.Channel, args *HeartBeatArguments) *corev1.Pod {

	if channel == nil || channel.Status.Sinkable.DomainInternal == "" {
		log.Printf("channel: %v", channel)
		return nil
	} else {
		log.Printf("channel: %v", channel)
	}

	remote := fmt.Sprintf("--remote=http://%s", channel.Status.Sinkable.DomainInternal)
	period := ""
	if args.Period > 0 {
		period = fmt.Sprintf("--period=%d", args.Period)
	}
	label := ""
	if args.Label != "" {
		period = fmt.Sprintf("--label=%s", args.Label)
	}

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: args.Name + "-er",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Containers: []corev1.Container{
				{
					Name:  "heartbeat",
					Image: image,
					Args: []string{
						remote, period, label,
					},
				},
			},
		},
	}

	return pod
}
