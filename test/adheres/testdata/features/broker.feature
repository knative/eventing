Feature: Conformance Test Broker
    Scenario Outline: A Broker <name> with Class <brokerClass>.
        Given a new environment
            And a new Broker named "<name>" with class "<brokerClass>"
        When running test "AsMiddleware"
        Then all PASS

        Examples:
            | name          | brokerClass     |
            | default       | default         |
            | channelbroker | MTChannelBroker |

    # assumption: My cluster has an empty namespace named target-env
    Scenario Outline: My Namespace, A Broker <name> with Class <brokerClass>.
        Given an existing namespace "target-env" for environment
            And a new Broker named "<name>" with class "<brokerClass>"
        When running test "AsMiddleware"
        Then all PASS
        Examples:
            | name          | brokerClass     |
            | default       | default         |
            | channelbroker | MTChannelBroker |

    # assumption: My cluster has an empty namespace named target-env2 with a running Broker mybroker
    Scenario Outline: My Broker.
        Given an existing namespace "target-env2" for environment
            And an existing Broker named "mybroker"
        When running test "AsMiddleware"
        Then all PASS
