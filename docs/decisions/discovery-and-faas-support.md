# Discovery and Supporting the FaaS Scenario (for 0.8.0)

Decision date: 5 August 2019

_Decision being recorded for 0.8.0, implementation expected by 0.9.0_

## Background

This file is being committed to record the design changes to solve issue
[#1381](https://github.com/knative/eventing/issues/1381). In this issue, we
[identified 3 scenarios](https://docs.google.com/document/d/1DpiSL2dUcYS2n7yXOIG5LJwyIC1lY9q_W8-56U1SvKM/edit?hl=en#heading=h.wv6g4odss7hh)
that are in scope for Knative eventing.

1. __FaaS scenario__ - Similar to: AWS Lambda and Google Cloud Functions. As a
 developer, I want to deliver from a managed cloud service, to a target Service
 (or Addressable). Managed cloud services are services like AWS S3, Google Cloud
 Storage, Pub/Sub, etc.

1. __Event-Driven scenario__ - Usually achieved using Kafka, Google Pub/Sub, AWS
SNS, etc
 * As an event-producer developer, I want to define custom on-cluster events and
   send them to a central location.
 * As an event-consumer developer, I want to consume these custom on-cluster
   events, without directly specifying the sender of the event (i.e., the
   Service generating the event).

1. __Black-box integration scenario__
 * As a central platform team, I want to set up an integration with external,
   black box software (e.g. SAP, Salesforce, etc), so that my development team
   can create extensions.
 * As a developer, I want to connect to external black box software without
   having to configure the connection

## Discovery

While working through this issue, we agreed upon the need for enhanced
discovery in order to enable the desired use cases.

We will move forward with event source based discovery. Design and
implementation will be tracked in
[#1550](https://github.com/knative/eventing/issues/1550).

## FaaS scenario

When using Broker + Trigger: We will make implementation changes that allow
for a trigger to be written so that it only receives events from a specific
importer. The implementation and design is tracked in
[#1555](https://github.com/knative/eventing/issues/1555).
