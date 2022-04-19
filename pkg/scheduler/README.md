# Knative Eventing Multi-Tenant Scheduler with High-Availability

An eventing source instance (for example, [KafkaSource](https://github.com/knative-sandbox/eventing-kafka/tree/main/pkg/source), [RedisStreamSource](https://github.com/knative-sandbox/eventing-redis/tree/main/source), etc) gets materialized as a virtual pod (**vpod**) and can be scaled up and down by increasing or decreasing the number of virtual pod replicas (**vreplicas**).  A vreplica corresponds to a resource in the source that can replicated for maximum distributed processing (for example, number of consumers running in a consumer group).

The vpod multi-tenant [scheduler](#1scheduler) is responsible for placing vreplicas onto real Kubernetes pods. Each pod is limited in capacity and can hold a maximum number of vreplicas. The scheduler takes a list of (source, # of vreplicas) tuples and computes a set of Placements. Placement info are added to the source status.

Scheduling strategies rely on pods having a sticky identity (StatefulSet replicas) and the current [State](#4state-collector) of the cluster.

When a vreplica cannot be scheduled it is added to the list of pending vreplicas. The [Autoscaler](#3autoscaler) monitors this list and allocates more pods for placing it.

To support high-availability the scheduler distributes vreplicas uniformly across failure domains such as zones/nodes/pods containing replicas from a StatefulSet.

## General Scheduler Requirements

1. High Availability: Vreplicas for a source must be evenly spread across domains to reduce impact of failure when a zone/node/pod goes unavailable for scheduling.*

2. Equal event consumption: Vreplicas for a source must be evenly spread across adapter pods to provide an equal rate of processing events. For example, Kafka broker spreads partitions equally across pods so if vreplicas aren’t equally spread, pods with fewer vreplicas will consume events slower than others.

3. Pod spread not more than available resources: Vreplicas for a source must be evenly spread across pods such that the total number of pods with placements does not exceed the number of resources available from the source (for example, number of Kafka partitions for the topic it's consuming from). Else, the additional pods have no resources (Kafka partitions) to consume events from and could waste Kubernetes resources.

* Note: StatefulSet anti-affinity rules guarantee new pods to be scheduled on a new zone and node.

## Components:

### 1.Scheduler
The scheduling framework has a pluggable architecture where plugins are registered and compiled into the scheduler. It allows many scheduling features to be implemented as plugins, while keeping the scheduling "core" simple and maintainable.

Scheduling happens in a series of stages:

  1. **Filter**: These plugins (predicates) are used to filter out pods where a vreplica cannot be placed. If any filter plugin marks the pod as infeasible, the remaining plugins will not be called for that pod. A vreplica is marked as unschedulable if no pods pass all the filters.

  2. **Score**: These plugins (priorities) provide a score to each pod that has passed the filtering phase. Scheduler will then select the pod with the highest weighted scores sum.

Scheduler must be Knative generic with its core functionality implemented as core plugins. Anything specific to an eventing source will be implemented as separate plugins (for example, number of Kafka partitions)

It allocates one vreplica at a time by filtering and scoring schedulable pods.

A vreplica can be unschedulable for several reasons such as pods not having enough capacity, constraints cannot be fulfilled, etc.

### 2.Descheduler

Similar to scheduler but has its own set of priorities (no predicates today).

### 3.Autoscaler

The autoscaler scales up pod replicas of the statefulset adapter when there are vreplicas pending to be scheduled, and scales down if there are unused pods. It takes into consideration a scaling factor that is based on number of domains for HA.

### 4.State Collector

Current state information about the cluster is collected after placing each vreplica and during intervals. Cluster information include computing the free capacity for each pod, list of schedulable pods (unschedulable pods are pods that are marked for eviction for compacting, and pods that are on unschedulable nodes (cordoned or unreachable nodes), number of pods (stateful set replicas), number of available nodes, number of zones, a node to zone map, total number of vreplicas in each pod for each vpod (spread), total number of vreplicas in each node for each vpod (spread),  total number of vreplicas in each zone for each vpod (spread), etc.

### 5.Reservation

Scheduler also tracks vreplicas that have been placed (ie. scheduled) but haven't been committed yet to its vpod status. These reserved veplicas are taken into consideration when computing cluster's state for scheduling the next vreplica.

### 6.Evictor

Autoscaler periodically attempts to compact veplicas into a smaller number of free replicas with lower ordinals. Vreplicas placed on higher ordinal pods are evicted and rescheduled to pods with a lower ordinal using the same scheduling strategies.

## Scheduler Profile

### Predicates:

1. **PodFitsResources**: check if a pod has enough capacity [CORE]

2. **NoMaxResourceCount**: check if total number of placement pods exceed available resources  [KAFKA]. It has an argument `NumPartitions` to configure the plugin with the total number of Kafka partitions.

3. **EvenPodSpread**: check if resources are evenly spread across pods [CORE]. It has an argument `MaxSkew` to configure the plugin with an allowed skew factor.

### Priorities:

1. **AvailabilityNodePriority**: make sure resources are evenly spread across nodes [CORE]. It has an argument `MaxSkew` to configure the plugin with an allowed skew factor.

2. **AvailabilityZonePriority**: make sure resources are evenly spread across zones [CORE]. It has an argument `MaxSkew` to configure the plugin with an allowed skew factor.

3. **LowestOrdinalPriority**: make sure vreplicas are placed on free smaller ordinal pods to minimize resource usage [CORE]

**Example ConfigMap for config-scheduler:**

```
data:
  predicates: |+
                  [
                    {"Name": "PodFitsResources"},
                    {"Name": "NoMaxResourceCount",
                    "Args": "{\"NumPartitions\": 100}"},
                    {"Name": "EvenPodSpread",
                    "Args": "{\"MaxSkew\": 2}"}
                  ]
  priorities: |+
                  [
                    {"Name": "AvailabilityZonePriority",
                    "Weight": 10,
                    "Args":  "{\"MaxSkew\": 2}"},
                    {"Name": "LowestOrdinalPriority",
                    "Weight": 2}
                  ]
```

## Descheduler Profile:

### Priorities:

1. **RemoveWithAvailabilityNodePriority**: make sure resources are evenly spread across nodes [CORE]

2. **RemoveWithAvailabilityZonePriority**: make sure resources are evenly spread across zones [CORE]

3. **HighestOrdinalPriority**: make sure vreps are removed from higher ordinal pods to minimize resource usage [CORE]

**Example ConfigMap for config-descheduler:**

```
data:
  priorities: |+
                  [
                    {"Name": "RemoveWithEvenPodSpreadPriority",
                    "Weight": 10,
                    "Args": "{\"MaxSkew\": 2}"},
                    {"Name": "RemoveWithAvailabilityZonePriority",
                    "Weight": 10,
                    "Args":  "{\"MaxSkew\": 2}"},
                    {"Name": "RemoveWithHighestOrdinalPriority",
                    "Weight": 2}
                  ]
```

## Normal Operation

1. **Busy scheduler**:

Scheduler can be very busy allocating the best placements for multiple eventing sources at a time using the scheduler predicates and priorities configured. During this time, the cluster could see statefulset replicas increasing, as the autoscaler computes how many more pods are needed to complete scheduling successfully. Also, the replicas could be decreasing during idle time, either caused by less events flowing through the system, or the evictor compacting vreplicas placements into a smaller number of pods or the deletion of event sources. The current placements are stored in the eventing source's status field for observability.

2. **Software upgrades**:

We can expect periodic software version upgrades or fixes to be performed on the Kubernetes cluster running the scheduler or on the Knative framework installed. Either of these scenarios could involve graceful rebooting of nodes and/or reapplying of controllers, adapters and other resources.

All existing vreplica placements will still be valid and no rebalancing will be done by the vreplica scheduler.
(For Kafka, its broker may trigger a rebalancing of partitions due to consumer group member changes.)

TODO: Measure latencies in events processing using a performance tool (KPerf eventing).

3. **No more cluster resources**:

When there are no resources available on existing nodes in the cluster to schedule more pods and the autoscaler continues to scale up replicas, the new pods are left in a Pending state till cluster size is increased. Nothing to do for the scheduler until then.

## Disaster Recovery

Some failure scenarios are described below:

1. **Pod failure**:

When a pod/replica in a StatefulSet goes down due to some reason (but its node and zone are healthy), a new replica is spun up by the StatefulSet with the same pod identity (pod can come up on a different node) almost immediately.

All existing vreplica placements will still be valid and no rebalancing will be done by the vreplica scheduler.
(For Kafka, its broker may trigger a rebalancing of partitions due to consumer group member changes.)

TODO: Measure latencies in events processing using a performance tool (KPerf eventing).

2. **Node failure (graceful)**:

When a node is rebooted for upgrades etc, running pods on the node will be evicted (drained), gracefully terminated and rescheduled on a different node. The drained node will be marked as unschedulable by K8 (`node.Spec.Unschedulable` = True) after its cordoning.

```
k describe node knative-worker4
Name:               knative-worker4
CreationTimestamp:  Mon, 30 Aug 2021 11:13:11 -0400
Taints:            none
Unschedulable:      true
```

All existing vreplica placements will still be valid and no rebalancing will be done by the vreplica scheduler.
(For Kafka, its broker may trigger a rebalancing of partitions due to consumer group member changes.)

TODO: Measure latencies in events processing using a performance tool (KPerf eventing).

New vreplicas will not be scheduled on pods running on this cordoned node.

3. **Node failure (abrupt)**:

When a node goes down unexpectedly due to some physical machine failure (network isolation/ loss, CPU issue, power loss, etc), the node controller does the following few steps

Pods running on the failed node receives a NodeNotReady Warning event

```
k describe pod kafkasource-mt-adapter-5 -n knative-eventing
Name:         kafkasource-mt-adapter-5
Namespace:    knative-eventing
Priority:     0
Node:         knative-worker4/172.18.0.3
Tolerations:                 node.kubernetes.io/not-ready:NoExecute op=Exists for 300s                 node.kubernetes.io/unreachable:NoExecute op=Exists for 300s

Events:
  Type     Reason        Age    From               Message
  ----     ------        ----   ----               -------
  Normal   Scheduled     11m    default-scheduler  Successfully assigned knative-eventing/kafkasource-mt-adapter-5 to knative-worker4
  Normal   Pulled        11m    kubelet            Container image
  Normal   Created       11m    kubelet           Created container receive-adapter
  Normal   Started       11m    kubelet            Started container receive-adapter
  Warning  NodeNotReady  3m48s  node-controller    Node is not ready
```

Failing node is tainted with the following Key:Condition: by the node controller if the node controller has not heard from the node in the last node-monitor-grace-period (default is 40 seconds)

```
k describe node knative-worker4
Name:               knative-worker4
Taints:             node.kubernetes.io/unreachable:NoExecute
                       node.kubernetes.io/unreachable:NoSchedule
Unschedulable:      false
 Events:
  Type    Reason              Age    From     Message
  ----    ------              ----   ----     -------
  Normal  NodeNotSchedulable  5m42s  kubelet  Node knative-worker4 status is now: NodeNotSchedulable
  Normal  NodeSchedulable     2m31s  kubelet  Node knative-worker4 status is now: NodeSchedulable
```

```
k get nodes
NAME                    STATUS     ROLES                  AGE     VERSION
knative-control-plane   Ready      control-plane,master   7h23m   v1.21.1
knative-worker          Ready      <none>                 7h23m   v1.21.1
knative-worker2         Ready      <none>                 7h23m   v1.21.1
knative-worker3         Ready      <none>                 7h23m   v1.21.1
knative-worker4         NotReady   <none>                 7h23m   v1.21.1
```

After a timeout period (`pod-eviction-timeout` == 5 mins (default)), the pods move to the Terminating state.

Since statefulset now has a `terminationGracePeriodSeconds: 0` setting, the terminating pods are immediately restarted on another functioning Node. A new replica is spun up with the same ordinal.

During the time period of the failing node being unreachable (~5mins), vreplicas placed on that pod aren’t available to process work from the eventing source. (Theory) Consumption rate goes down and Kafka eventually triggers rebalancing of partitions. Also, KEDA will scale up the number of consumers to resolve the processing lag. A scale up will cause the Eventing scheduler to rebalance the total vreplicas for that source on available running pods.

4. **Zone failure**:

All nodes running in the failing zone will be unavailable for scheduling. Nodes will either be tainted with `unreachable` or Spec’ed as `Unschedulable`
See node failure scenarios above for what happens to vreplica placements.

## References:

* https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/
* https://github.com/kubernetes-sigs/descheduler
* https://kubernetes.io/docs/reference/scheduling/policies/
* https://kubernetes.io/docs/reference/config-api/kube-scheduler-policy-config.v1
* https://github.com/virtual-kubelet/virtual-kubelet#how-it-works
* https://github.com/kubernetes/enhancements/tree/master/keps/sig-scheduling/624-scheduling-framework
* https://medium.com/tailwinds-navigator/kubernetes-tip-how-statefulsets-behave-differently-than-deployments-when-node-fails-d29e36bca7d5
* https://kubernetes.io/docs/concepts/architecture/nodes/#node-controller


---

To learn more about Knative, please visit the
[/docs](https://github.com/knative/docs) repository.

This repo falls under the
[Knative Code of Conduct](https://github.com/knative/community/blob/master/CODE-OF-CONDUCT.md)
