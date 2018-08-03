# Slack - Message Dumper

This is a simple program which simply writes any messages received to standard
out, to be viewed in logs. It is run as a Knative `Service`.

## Prerequisites

1.  Follow all the steps in the Prerequisites and Setup sections of
    [sample/slack](../README.md).

## Installation

1.  Install the message-dumper `Service`.

    ```shell
    ko apply -f sample/slack/message_dumper/service.yaml
    ```

1.  Edit `sample/slack/message_dumper/flow.yaml`, replacing
    `spec.trigger.resource` with a nickname of your Slack App. It will be used
    to name the receive adapter's `Service`.

    `Note: If you change from the default 'keventing-app', you will need to
    replace that string in all the subsequent commands.`

    -   If you want to use a different service account, then replace
        `spec.serviceAccountName` as well.

1.  Install the flow, hooking up the `EventSource` to the `Service`.

    ```shell
    kubectl apply -f sample/slack/message_dumper/flow.yaml
    ```

1.  We can see all the resources created for this example:

    ```shell
    # Show the Service we created.
    kubectl get services.serving.knative.dev message-dumper -oyaml
    echo '---'

    # Show the Flow we created.
    kubectl get flows.flows.knative.dev slack-flow -oyaml
    echo '---'

    # Show the Feed the Flow created.
    kubectl get feeds.feeds.knative.dev slack-flow -oyaml
    echo '---'

    # Show the Channel the Flow created.
    kubectl get channels.channels.knative.dev slack-flow -oyaml
    echo '---'

    # Show the Subscriptions the Feed created.
    kubectl get subscriptions.channels.knative.dev slack-flow -oyaml
    echo '---'

    # Show the Receive Adapter the Feed created.
    kubectl get services.serving.knative.dev slack-keventing-app-rcvadptr -oyaml
    ```

1.  Set the Slack App's 'Request URL'.

    1.  Find the receive adapter's URL.

        ```shell
        kubectl get services.serving.knative.dev slack-keventing-app-rcvadptr -o='jsonpath={.status.domain}'
        ```

    1.  Go to the 'Event Subscriptions' settings page of your Slack App and
        enter the URL returned from the previous command into the 'Request URL'
        box (make sure it has a leading http://).

        -   It should say 'Verified' after a few seconds.

## Running

1.  Write a message in a public Slack channel, in a Slack workspace with your
    Slack App installed.

    `Note: If you are using kail in the following steps, then you will need to
    repeat this step after kail has started.`

1.  View the logs generated to see that the event was received appropriately.

    *   `kail`

        ```shell
        kail --label serving.knative.dev/configuration=slack-keventing-app-rcvadptr -c user-container
        ```

        *   Repeat the previous instruction to send a Slack message.

    *   `kubectl logs`

        1.  Find the current receive adapter's `Pods`.

            ```shell
            kubectl get pods --selector=serving.knative.dev/configuration=slack-keventing-app-rcvadptr
            ```

            -   The only pod for me is
                `slack-keventing-app-rcvadptr-00001-deployment-6bf54db95d-5v5h5`.

        1.  Look at the logs of the pod.

            ```shell
            kubectl logs slack-keventing-app-rcvadptr-00001-deployment-6bf54db95d-5v5h5 user-container
            ```

1.  View the logs generated to see that the event was routed appropriately.

    *   `kail`

        ```shell
        kail --label serving.knative.dev/configuration=message-dumper -c user-container
        ```

        *   Repeat the previous instruction to send a Slack message.

    *   `kubectl logs`

        1.  Find the current message dumper's `Pods`.

            ```shell
            kubectl get pods --selector=serving.knative.dev/configuration=message-dumper
            ```

            -   The only pod for me is
                `message-dumper-00001-deployment-7df2316773-343ab`.

        1.  Look at the logs of the pod.

            ```shell
            kubectl logs message-dumper-00001-deployment-7df2316773-343ab user-container
            ```

## Clean up

1.  Delete the `Flow`.

    ```shell
    kubectl delete -f sample/slack/message_dumper/flow.yaml
    ```

    -   Due to a bug in `Flow` deletion, the `Flow`'s `Feed` continues to exist.
        Delete it.

        ```shell
        kubectl delete feed slack-flow
        ```

1.  Delete the message-dumper `Service`.

    ```shell
    ko delete -f sample/slack/message_dumper/serving.yaml
    ```

1.  Follow the generic Slack [clean up instructions](../README.md#clean-up).
