# Knative Personas

When discussing user actions, it is often helpful to
[define specific user roles](<https://en.wikipedia.org/wiki/Persona_\(user_experience\)>)
who might want to do the action.

## Knative Events

Event generation and consumption is a core part of the serverless (particularly
function as a service) computing model. Event generation and dispatch enables
decoupling of event producers from consumers.

### Event consumer (developer)

An event consumer may be a software developer, or may be an integrator which is
reusing existing packaged functions to build a workflow without writing code.

User stories:

- Determine what event sources are available
- Trigger my service when certain events happen (event feed)
- Filter events from a provider

### Event producer

An event producer owns a data source or system which produces events which can
be acted on by event consumers.

User stories:

- Publish events
- Control who can create Feeds

### System Integrator

System Integrators are typically of three varieties. They are producing new
Channel implementations, or new Event Source implementations, or new Broker
implementations.

User stories:

- Create a new Event Source (creating a bridge between an existing system and
  Knative Eventing)
- Create a new Channel implementation
- Create a new Broker implementation

## Contributors

Contributors are an important part of the Knative project. As such, we will also
consider how various infrastructure encourages and enables contributors to the
project, as well as the impact on end-users.

- Hobbyist or newcomer
- Motivated user
- Corporate (employed) maintainer
- Consultant

User stories:

- Check out the code
- Build and run the code
- Run tests
- View test status
- Run performance tests
