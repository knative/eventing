Eventing Roadmap

At the end of each milestone, there should be a new or updated demo in
the samples directory.

# 0.1 CloudEvents Demo

A. Ability to register an event sources and specify which events may be
   generated from that source
   [Proposal](https://github.com/elafros/eventing/issues/39)
B. kubctl command to set up a trigger where a specific event will cause a
  specific action powered by a specific processor
C. Example source generating an event
D. Example action
E. Events conform to [CloudEvents](https://github.com/cloudevents/spec)
   0.1 specification

# 0.2 Friendly Repo

A. Easy installation [issue#51](https://github.com/elafros/eventing/issues/51)
B. We have clear guidelines for how we use Prow
C. All the contributing info is tested and someone new to the repo has made a
   PR by following the directions

# 0.3 Functions

A. An event can trigger an Elafros function [issue#52](https://github.com/elafros/eventing/issues/52)
B. A local event can trigger a Google Cloud Function
C. A Google source can trigger a local action. Likely this will use some
   scaffolding on the Google side to validate the interaction between systems,
   such as deploying a Cloud Function that forwards the Google Event to the
   Elafros cluster.

# 0.4 Event Persistence

A. Failed event handling can be retried
B. An Event Source can be added to a cluster without recompile
C. There is a well-defined interface that allows the use of popular event
   brokers for persistence, such as Kafka and Rabbit Mq

# 0.5 Piplelines

A. Pipeline the output of one action into the input of another

# 0.6 Streams

A. an action may accept a stream interface to process many events at once
B. Events are delivered in order
