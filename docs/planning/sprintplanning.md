# KNative Eventing sprint planning

This document summarizes project planning mechanics for KNative Eventing releases.

## How to manage Bugs?
- Submit new bugs by creating a new issue of type “Report Bug”.
- Please fill in as much detail as possible in the bug template and propose priority and severity
- All new bugs will be labelled “triage/needstriage” and kind/bug as part of the template
- Bugs will be triaged weekly (this could be modified based on incoming bugs rate). And upon triage will be labeled triage/accepted, or triage/rejected (and closed) with appropriate priority (priority/*) and severity (severity/*)
- priority/p1 and priority/p2 bugs will be planned for next release cycle to keep low engineering debt.
- priority/p0 is reserved for critical bugs that needs to be addressed immediately in current release at the cost of currently planned items.
- WG lead will drive this and his/her decision will be final

## How to manage Story\Feature?
- Story/Feature typically spans 1-2 weeks for one engineer.
To propose a new feature,  create a new issue of type “Feature Request”
- Please fill in as much detail as possible in the Feature template and propose a release cycle
- All new features will be labelled “kind/feature”, and “triage/needstriage”
- All new proposals are discussed either in WG meeting, or on slack, or dedicated meetings if needed at any point scheduled by the proposer. 
- By R-2w’s WG meeting all new proposals should be vetted and verified that they fit the [Planning criteria](#planning-criteria). Features that do not fit this criteria will not be considered in the current release planning cycle and hence early discussion is encouraged instead of waiting for R-2w.
- All eligible proposals will then be snapshotted into the voting document and open for voting till R-1w WG meeting. In R-1w WG meeting top features based on votes will be planned for next release. 
> We need some kind of estimates for each feature for release planning. Shall we adopt story points on fibonacci scale or t-shirt sizing? My vote goes to story points on fibonacci scale to avoid spending a lot of time on estimation, which is a common pitfall.
- Selected features will be labeled “triage/accepted”. Rest will be labelled “triage/ice-boxed” and will be eligible for next planning cycle. Add these features to correct milestone and sprint specific project for tracking

## How to manage Epic/Scenario?
- Epic/Scenario typically spans 1-3 months or even more in some cases.
- For new proposals these will be created and triaged/refined just like Story/Feature. However, after triaging these need to be broken down into Features for planning and voting.
- Github doesn’t allow nested Issues, hence the only good way to track these are by creating Projects and adding the broken down features as issues to this project. An issue can be part of multiple projects and hence we can get a Scenario\Epic view as well as a Sprint view through respective projects.

## Planning criteria:
- Features are well defined and scoped with clear exit criteria. No feature creeps once release starts
- Features can be implemented in <2w by one engineer. In case of Epic/Scenario, it could be broken down into <2w features with clear exit criteria.
- The engineering team should be confident that any engineer can implement the feature based on the content in the feature *(This is similar to refinement in a scrum, but doesn’t have to be as strict as a functional specification)*.

## Organizing with Github Projects 
- New Project per milestone
- New Project per any Story/Scenario
- Any other projects that help organize and will be created when needed.

## Pending work and proposed changes:
- Decide estimation technique (story points on fibonacci, t-shirt size etc)
- Create features and bugs for 0.6 release.
- Create appropriate templates and verify if things will work with our current permission model and Prow - I have experimented all this in my private repo, but do not have right permissions in eventing repo. Will need some help here - @Grant?
- Create following labels:
    - triage/needstriage
    - triage/accepted
    - kind/rejected
    - triage/ice-boxed
    - kind/feature
    - priority/0 - Critical issues that needs to be addressed right away in current release
    - priority/1 - Highest priority
    - priority/2
    - priority/3
    - priority/4 - lowest priority
    - severity/1
    - severity/2
    - severity/3
    - severity/4
- Need to define Priorities and Severities. Is there any norm followed in KNative?

## Future work:
- If the above process works for next couple of releases and team likes it then we move to a 2 week sprint model rather than 6 weeks.
