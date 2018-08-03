# Slack

This example creates a Slack event source. 

## Prerequisites

### Create a Slack App

1. foobar

### Install Knative

1.  [Setup your development environment](../../DEVELOPMENT.md#getting-started).

1.  [Start Knative Serving](../../README.md#start-knative).

1.  [Start Knative Eventing](../../README.md#start-eventing)

1.  Create a cluster bus. You **must** change the Kind to ClusterBus in
    stub-sub.yaml file.

    ```shell
    ko apply -f config/buses/stub/stub-bus.yaml
    ```

## Setup

### Service account

Because the Receive Adapter needs to run a service, you need to specify what
Service Account should be used in the target namespace for running the Receive
Adapter. Flow.Spec has a field that allows you to specify this. By default it
uses "default" for feed which typically has no privileges, but this feed
requires standing up a service, so you need to either use an existing
Service Account with appropriate privileges or create a new one. This example
creates a Service Account and grants it cluster admin access. You probably
wouldn't want to do that in production settings, but for this example it will
suffice just fine.

1.  Create and give permissions to the service account.

    ```shell
    kubectl apply -f sample/slack/auth.yaml
    ```

### EventSource and EventType

1.  Install the EventSource.

    ```shell
    ko apply -f pkg/sources/slack/eventsource.yaml
    ```
    
1.  Install the EventType.

    ```shell
    ko apply -f pkg/sources/slack/eventtype.yaml
    ```
    
### Slack Secrets

In order to verify the requests come from Slack, we need to save our app's
verification token into a secret.

1.  Find your Slack App's verification token on the 'Basic Information' page of
    your Slack App's settings.
    
1.  Edit `sample/slack/slacksecret.yaml`, replacing "<YOUR PERSONAL TOKEN FROM
    SLACK>" with the actual token found in step 1.
    
1.  Create the secret.

    ```shell
    kubectl apply -f sample/slack/slacksecret.yaml
    ```

## Flow

Two samples are available. Both use the same event source, but do different
things in response to the events.

*   [Message Dumper](message_dumper/README.md) - A simple program that writes
    received messages to standard out.
    
*   [Slack Responder](slack_responder/README.md) - Echoes messages sent in
    public channels back to those channels.

# Clean up

1.  Clean up anything unique to either sample, by following the sample's cleanup
    instructions first.
    
1.  Delete the secret.

    ```shell
    kubectl delete -f sample/slack/slacksecret.yaml
    ```

1.  Delete the EventSource and EventType.

    ```shell
    ko delete -f pkg/sources/slack/eventsource.yaml
    ko delete -f pkg/sources/slack/eventype.yaml
    ```
    
1.  Delete the service account and its permissions.

    ```shell
    kubectl delete -f sample/slack/auth.yaml
    ```
