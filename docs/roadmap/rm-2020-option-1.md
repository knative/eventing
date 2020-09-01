# 2020 Eventing Roadmap

## References

[Personas and User Stories](../personas.md):

Knative Eventing caters to the following personas when developing different
features

1. Event consumer (developer)
1. Event producer
1. System Integrator
1. Contributors

## Areas of Interest and Requirements

1. **API Maturity**. Description TBD
1. **Production-ready Brokers**. Description TBD
1. **Knative Inbound Authorization**. Description TBD
1. **Serverless Ecosystem**. Description TBD
1. **Test and Release**. Description TBD
1. **Multi-Tenancy**. Description TBD

### API Maturity

1. **Define Broker spec and conformance tests** including data plane
   specification https://github.com/knative/eventing/issues/2614. (issue TBD)
1. **Graduate to V1 APIs** This also mplies increasing of conformance test for
   channels, broker and sources. (issue TBD)

### Production-ready Brokers

1. **Minimal Footprint (CPU/RAM)** The underlying platform components have a
   minimal (near zero) footprint (CPU/RAM) in the cluster when eventing is not
   being used. (issue TBD)
1. **Observability** Improvements to E2E tracing, metrics, and logs. (issue TBD)
1. **Lossless upgrades** Brokers never go unavailable during upgrade and no
   events get lost. (issue TBD)

### Knative Inbound Authorization

1. **Application Level Authorization** (Not limited to eventing) Standardize a
   Knative way to do application level authorization and make the policy
   implementation/provider extensible (e.g. GCP IAM, Azure AD, etc.). (issue
   TBD)
1. **Unauthorized Requests Denial** (Not limited to eventing) For Knative
   services, unauthorized requests will not trigger user pods creation.(issue
   TBD)
1. **Events Authorization Policies** Any “authorizable” service will be able to
   create policies to control what events can be accepted.(issue TBD)
1. **Securing Eventing Documentation** Documentation of best practices for
   securing an eventing environment (likely combined with service mesh and
   network policy).(issue TBD)

### Serverless Ecosystem

1. **More Sources** Make it easier to write sources. (issue TBD)
1. **Data Plane Scalability** TBD. (issue TBD)
1. **Second party events** Integration with registry and discovery
   (CloudEvents). (issue TBD)
1. **Go, and Java CloudEvents sdks** Golang and Java CloudEvents SDKs should be
   in a good shape both to implement eventing components and to be used by the
   end user to interact with eventing. (issue TBD)

### Test and Release

1. **Robust CI/CD release pipeline** Upgrade tests for Knative Eventing
   (Stability). (issue TBD)

### Multi-Tenancy

1. **MT PingSource component** TBD
1. **MT GithubSource component** TBD
1. **MT Brokers** TBD

## What We Are Not Doing Yet

Are there things we are explicitly leaving off the roadmap but we might do
exploratory work to set them up for later development?
