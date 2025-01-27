# Knative Eventing Multi-Tenant Scheduler with High-Availability

An eventing source instance (for example, KafkaSource, etc) gets materialized as a virtual pod (*
*vpod**) and can be scaled up and down by increasing or decreasing the number of virtual pod
replicas (**vreplicas**). A vreplica corresponds to a resource in the source that can replicated for
maximum distributed processing (for example, number of consumers running in a consumer group).

The vpod multi-tenant [scheduler](#scheduler) is responsible for placing vreplicas onto real
Kubernetes pods. Each pod is limited in capacity and can hold a maximum number of vreplicas. The
scheduler takes a list of (source, # of vreplicas) tuples and computes a set of Placements.
Placement info are added to the source status.

Scheduling strategies rely on pods having a sticky identity (StatefulSet replicas) and the
current [State](#state-collector) of the cluster.

## Components:

### Scheduler

The scheduler allocates as many as vreplicas as possible into the lowest possible StatefulSet
ordinal
number before triggering the autoscaler when no more capacity is left to schedule vpods.

### Autoscaler

The autoscaler scales up pod replicas of the statefulset adapter when there are vreplicas pending to
be scheduled, and scales down if there are unused pods.

### State Collector

Current state information about the cluster is collected after placing each vreplica and during
intervals. Cluster information include computing the free capacity for each pod, list of schedulable
pods (unschedulable pods are pods that are marked for eviction for compacting, number of pods (
stateful set replicas), total number of vreplicas in each pod for each vpod (spread).

### Evictor

Autoscaler periodically attempts to compact veplicas into a smaller number of free replicas with
lower ordinals. Vreplicas placed on higher ordinal pods are evicted and rescheduled to pods with a
lower ordinal using the same scheduling strategies.

## Normal Operation

1. **Busy scheduler**:

Scheduler can be very busy allocating the best placements for multiple eventing sources at a time
using the scheduler predicates and priorities configured. During this time, the cluster could see
statefulset replicas increasing, as the autoscaler computes how many more pods are needed to
complete scheduling successfully. Also, the replicas could be decreasing during idle time, either
caused by less events flowing through the system, or the evictor compacting vreplicas placements
into a smaller number of pods or the deletion of event sources. The current placements are stored in
the eventing source's status field for observability.

2. **Software upgrades**:

We can expect periodic software version upgrades or fixes to be performed on the Kubernetes cluster
running the scheduler or on the Knative framework installed. Either of these scenarios could involve
graceful rebooting of nodes and/or reapplying of controllers, adapters and other resources.

All existing vreplica placements will still be valid and no rebalancing will be done by the vreplica
scheduler.
(For Kafka, its broker may trigger a rebalancing of partitions due to consumer group member
changes.)

3. **No more cluster resources**:

When there are no resources available on existing nodes in the cluster to schedule more pods and the
autoscaler continues to scale up replicas, the new pods are left in a Pending state till cluster
size is increased. Nothing to do for the scheduler until then.

## Disaster Recovery

Some failure scenarios are described below:

1. **Pod failure**:

When a pod/replica in a StatefulSet goes down due to some reason (but its node and zone are
healthy), a new replica is spun up by the StatefulSet with the same pod identity (pod can come up on
a different node) almost immediately.

All existing vreplica placements will still be valid and no rebalancing will be done by the vreplica
scheduler.
(For Kafka, its broker may trigger a rebalancing of partitions due to consumer group member
changes.)

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
