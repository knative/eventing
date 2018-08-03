# Slack - Slack Responder

This is a program which echoes what user's say in a public Slack channel back to
them. It is run as a Knative `Service`.

## Prerequisites

1.  Follow all the steps in the Prerequisites and Setup sections of 
    [sample/slack](../README.md).
    
1.  Add the `chat:write:bot` scope to your Slack App on Slack's 
    'OAuth & Permissions' settings page.
    
1.  Add a 'Bot User' to your Slack App on Slack's 'Bot Users' settings page.
    
## Installation

### Bot OAuth Access Token

We already have the ability to verify Slack events, but now we need the ability
to send commands back to Slack.

1.  Find your Slack App's 'Bot User OAuth Access Token', on the 'OAuth &
    Permissions' page of your Slack App's settings.
    
1.  Edit `sample/slack/slack_responder/slackrespondersecret.yaml`, replacing 
    "<Your Bot User OAuth Access Token from Slack>" with the actual token found
     in step 1.
    
1.  Create the secret.

    ```shell
    kubectl apply -f sample/slack/slack_responder/slackrespondersecret.yaml
    ```

### Egress Permission

By default, our `Service` can't send requests to the internet. It will need to
send requests to `http://slack.com`. To give it permission we create an Istio 
`ServiceEntry`.

1.  Install the `ServiceEntry`.

    ```shell
    kubectl apply -f sample/slack/slack_responder/slack-serviceentry.yaml
    ```
    
### Service and Flow

1.  Install the slack-responder `Service`.

    ```shell
    ko apply -f sample/slack/slack_responder/service.yaml
    ```

1.  Edit `sample/slack/slack_responder/flow.yaml`, replacing 
    `spec.trigger.resource` with a nickname of your Slack App. It will be used 
    to name the receive adapter's `Service`.
    
    -   If you want to use a different service account, then replace 
        `spec.serviceAccountName` as well.
    
1.  Install the flow, hooking up the `EventSource` to the `Service`.

    ```shell
    kubectl apply -f sample/slack/slack_responder/flow.yaml
    ```

## Running

1.  We can see all the resources created for this example:

    ```shell
    # Show the Secret we created to allow us to send Events to Slack.
    kubectl get secrets slack-responder-secret -oyaml
    echo '---'
    
    # Show the Istio ServiceEntry we created.
    kubectl get serviceentries.networking.istio.io slack-responder-ext -oyaml
    echo '---'
    
    # Show the Service we created.
    kubectl get services.serving.knative.dev slack-responder -oyaml
    echo '---'
    
    # Show the Flow we created.
    kubectl get flows.flows.knative.dev slack-responder-flow -oyaml
    echo '---'
    
    # Show the Feed the Flow created.
    kubectl get feeds.feeds.knative.dev slack-responder-flow -oyaml
    echo '---'
    
    # Show the Channel the Flow created.
    kubectl get channels.channels.knative.dev slack-responder-flow -oyaml
    echo '---'
    
    # Show the Subscriptions the Feed created.
    kubectl get subscriptions.channels.knate.dev slack-responder-flow -oyaml
    ```
    
1.  Write a message in a public Slack channel, in a Slack workspace with your
    Slack App installed.
    
    `Note: If you are using kail in the following steps, then you will need to
    repeat this step after kail has started.`

1.  See your Bot write back to the channel 'You said: <Your message>'.

1.  View the logs generated to see that the event was received appropriately.
    
    *   The receive adapter is named based on the `Flow`'s `spec.resource`,
        using the format `slack-<flow-spec-resource>-rcvadptr`.
        
        *   The sample uses `keventing-app`, for a receive adapter named
            `slack-keventing-app-rcvadptr`. If yours is different replace it
            in the following instructions.
     
    *   `kail`
    
        ```shell
        kail --label serving.knative.dev/configuration=slack-keventing-app-rcvadptr -c user-container
        ```
        
        * Repeat the previous instruction to send a Slack message.
            
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
        
        * Repeat the previous instruction to send a Slack message.
            
    *   `kubectl logs`
     
        1.  Find the current receive adapter's `Pods`.
        
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
    kubectl delete -f sample/slack/slack_responder/flow.yaml
    ```
    
    -   Due to a bug in `Flow` deletion, the `Flow`'s `Feed` continues to exist.
        Delete it.
        
        ```shell
        kubectl delete feed slack-responder-flow
        ```
        
1.  Delete the slack-responder `Service`.

    ```shell
    ko delete -f sample/slack/slack_responder/serving.yaml
    ```
    
1.  Delete the Istio `ServiceEntry`.

    ```shell
    kubectl delete serviceentries.networking.istio.io slack-responder-ext
    ```
    
1.  Delete the `Secret`.

    ```shell
    kubectl delete secrets slack-responder-secret
    ```
    
1.  Follow the generic Slack [clean up instructions](../README.md#clean-up).
