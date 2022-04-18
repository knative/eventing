# Upgrade Tests

In order to get coverage for the upgrade process from an operator’s perspective,
we need an additional suite of tests that perform a complete knative upgrade.
Running these tests on every commit will ensure that we don’t introduce any
non-upgradeable changes, so every commit should be releasable.

This is inspired by kubernetes
[upgrade testing](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-testing/e2e-tests.md#version-skewed-and-upgrade-testing)
.

These tests are a pretty big hammer in that they cover more than just version
changes, but it’s one of the only ways to make sure we don’t accidentally make
breaking changes for now.

## Flow

We’d like to validate that the upgrade doesn’t break any resources (they still
propagate events) and doesn't break our installation (we can still update
resources).

At a high level, we want to do this:

1. Install the latest knative release.
1. Create some resources.
1. Install knative at HEAD.
1. Run any post-install jobs that apply for the release to be.
1. Test those resources, verify that we didn’t break anything.

To achieve that, we created an upgrade framework (knative.dev/pkg/test/upgrade).
This framework will enforce running upgrade tests in specific order and supports
continual verification of system under test. In case of Eventing it is:

1. Install the latest release from GitHub.
1. Run the `preupgrade` smoke tests.
1. Start `continual` tests that will propagate events in the background, while
   upgrading and downgrading.
1. Install at HEAD (`ko apply -f config/`) and run the post-install jobs.
1. Run the `postupgrade` smoke tests.
1. Install the latest release from GitHub.
1. Run the `postdowngrade` smoke tests.
1. Stop and verify `continual` tests, checking if every event propagated well.

## Tests

### Smoke test

This was stolen from the e2e tests as one of the simplest cases.

#### preupgrade, postupgrade, postdowngrade

Run the selected smoke test.

### Probe test

In order to verify that we don't have data-plane unavailability during our
control-plane outages (when we're upgrading the knative/eventing installation),
we run a prober test that continually sends events to a service during the
entire upgrade/downgrade process. When the upgrade completes, we make sure that
all of those events propagated at least once.

To achieve that
a [wathola tool](https://pkg.go.dev/knative.dev/eventing/test/upgrade/prober/wathola)
was prepared. It consists of 4 components: _sender_, _forwarder_, _receiver_,
and _fetcher_. _Sender_ is the usual Kubernetes deployment that publishes events
to the System Under Tests (SUT). By default, SUT is a default `broker`
with two triggers for each type of events being sent. _Sender_ will send events
with given interval. When it terminates (by either `SIGTERM`, or
`SIGINT`), a `finished` event is generated. _Forwarder_ is a knative serving
service that scales up from zero to receive the sent events and forward them to
given target which is the _receiver_ in our case. _Receiver_ is an ordinary
deployment that collects events from multiple forwarders and has an
endpoint `/report` that can be polled to get the status of received events. To
fetch the report from within the cluster _fetcher_ comes in. It's a simple one
time job, that will fetch the report from _receiver_ and print it on stdout as
JSON. That enables the test client to download _fetcher_ logs and parse the JSON
to get the final report.

Diagram below describe the setup:

```
                   K8s cluster                            |     Test machine
                                                          |
(deployment)        (ksvc)           (deployment)         |
+--------+       +-----------+       +----------+         |    +------------+
|        |       |           ++      |          |         |    |            |
| Sender |   +-->| Forwarder ||----->+ Receiver |         |    + TestProber |
|        |   |   |           ||      |          |<---+    |    |            |
+---+----+   |   +------------|      +----------+    |    |    +------------+
    |        |    +-----------+                      |    |
    | ```````|`````````````````````````````          |    |
    | `      |                            ` +---------+   |
    | `   +--+-----+       +---------+    ` |         |   |
    +----->        |       |         +-+  ` | Fetcher |   |
      `   | Broker | < - > | Trigger | |  ` |         |   |
      `   |        |       |         | |  ` +---------+   |
      `   +--------+       +---------+ |  `    (job)      |
      `    (default)        +----------+  `               |
      `              (SUT)                `
      `````````````````````````````````````
```

#### Probe test configuration

Probe test behavior can be influenced from outside without modifying its source
code. That can be beneficial if one would like to run upgrade tests in different
context. One such example might be running Eventing upgrade tests in place that
have Serving and Eventing both installed. In such environment one can set
environment variable `EVENTING_UPGRADE_TESTS_SERVING_USE` to enable usage of
ksvc forwarder (which is disabled by default):

```
$ export EVENTING_UPGRADE_TESTS_SERVING_USE=true
```

Any option, apart from namespace, in
[`knative.dev/eventing/test/upgrade/prober.Config`](https://github.com/knative/eventing/blob/022e281/test/upgrade/prober/prober.go#L52-L63)
struct can be influenced, by using `EVENTING_UPGRADE_TESTS_XXXXX` environmental
variable prefix (using
[kelseyhightower/envconfig](https://github.com/kelseyhightower/envconfig#usage)
usage).

#### Inspecting Zipkin traces for undelivered events

When tracing is enabled in the `config-tracing` config map in the system namespace
the prober collects traces for undelivered events. The traces are exported as json files
under the artifacts dir. Traces for each event are stored in a separate file.
Step event traces are stored as `$ARTIFACTS/traces/missed-events/step-<step_number>.json`
The finished event traces are stored as `$ARTIFACTS/traces/missed-events/finished.json`

Traces can be viewed as follows:
- Start a Zipkin container on localhost:
   ```
   $ docker run -d -p 9411:9411 ghcr.io/openzipkin/zipkin:2
   ```
- Send traces to the Zipkin endpoint:
   ```
   $ curl -v -X POST localhost:9411/api/v2/spans \
     -H 'Content-Type: application/json' \
     -d @$ARTIFACTS/traces/missed-events/step-<step_number>.json
   ```
- View traces in Zipkin UI at `http://localhost:9411/zipkin`
