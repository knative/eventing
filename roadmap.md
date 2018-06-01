# Eventing Roadmap

At the end of each milestone, there should be a new or updated demo linked from
the [README](README.md).

## 0.1 CloudEvents Demo

1. Ability to register an event sources and specify which events may be
   generated from that source
   [Proposal](https://github.com/knative/eventing/issues/39)
1. kubectl command to set up a trigger where a specific event will cause a
  specific action
1. Example source generating an event
1. Example action
1. Events conform to [CloudEvents](https://github.com/cloudevents/spec)
   0.1 specification

## 0.2 Friendly Repo

1. Easy installation [issue#51](https://github.com/knative/eventing/issues/51)
1. We have clear guidelines for how we use Prow
1. All the contributing info is tested and someone new to the repo has made a
   PR by following the directions

## 0.3 Functions

1. An event can trigger Knative function [issue#52](https://github.com/knative/eventing/issues/52)
1. A local event can trigger a Google Cloud Function
1. A Google source can trigger an action local within the Knative cluster.
   Likely this will use some scaffolding on the Google side to validate the
   interaction between systems, such as deploying a Cloud Function that
   forwards the Google Event to the Knative cluster.

## 0.4 Event Persistence

1. Events can be stored in a buffer / queue
1. Failed event handling can be retried
1. An Event Source can be added to a cluster without recompile
1. There is a well-defined interface that allows the use of popular event
   brokers for persistence, such as Kafka and Rabbit Mq

## 0.5 Pipelines

1. Pipeline the output of one action into the input of another
1. Pipelines can be a series of Functions

## 0.6 Streams

1. an action may accept a stream interface to process many events at once
1. Events are delivered in order
