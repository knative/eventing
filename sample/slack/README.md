# Slack

This example creates a Slack event source.

## Prerequisites

### Slack App

We have to create and configure a Slack App.

#### Create a Slack App

1.  Go to <https://api.slack.com/slack-apps>.

1.  Click 'Create a Slack App'.

1.  Follow the instructions to create the Slack App in a Slack Workspace.

### Install Knative

There needs to be a Kubernetes cluster with Knative Serving and Knative Eventing
installed.

1.  [Setup your development environment](../../DEVELOPMENT.md#getting-started).

## Setup

### Cluster Bus

1.  Create a cluster bus.

    1.  Edit `config/buses/stub/stub-bus.yaml`, changing the kind from `Bus` to
        `ClusterBus`.

    1.  Install the bus.

        ```shell
        ko apply -f config/buses/stub/stub-bus.yaml
        ```

### DNS

Because Slack will be sending events to us, we need to have a valid DNS entry
for them to call. Follow these
[instuctions](https://github.com/knative/docs/blob/master/serving/using-a-custom-domain.md)
to setup a custom domain. You can skip the 'Deploy an Application' and 'Local
DNS setup' sections.

### Service account

Because the Receive Adapter needs to run a service, you need to specify what
Service Account should be used in the target namespace for running the Receive
Adapter. Flow.Spec has a field that allows you to specify this. By default it
uses "default" for feed which typically has no privileges, but this feed
requires standing up a Knative `Service`, so you need to either use an existing
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

1.  Find your Slack App's verification token on the 'Basic Information' settings
    page of your Slack App.

1.  Edit `sample/slack/slacksecret.yaml`, replacing "\<YOUR PERSONAL TOKEN FROM
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
