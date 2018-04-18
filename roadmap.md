Eventing Roadmap

# 0.1

A. Ability to register an event sources and specify which events may be generated from that source [Proposal](https://github.com/elafros/eventing/issues/39)
B. kubctl command to set up a trigger where a specific event will cause a specific action
C. Example source generating an event
D. Example action
E. Events conform to [CloudEvents](https://github.com/cloudevents/spec) 0.1 specification

# 0.2

A. A local event can trigger a Google Cloud Function
B. A Google source can trigger a local action

# 0.3

A. Pipeline the output of one action into the input of another

# 0.4

A. an action may accept a stream interface to process many events at once