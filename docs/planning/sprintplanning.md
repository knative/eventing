# Knative Eventing sprint planning

This document summarizes project planning mechanics for Knative Eventing releases. Knative Eventing release is planned every six weeks or 6th Tuesday, with 0.5 planned on April 2, 2019 as reference. Current proposal is to align sprint with the release cycle of 6 weeks. Eventually this may change as we learn from the process.

## How to manage Bugs?
- Submit new bugs by creating a new issue of type “Bug”.
- Please fill the bug template and propose priority.
- All new bugs will be labelled “kind/bug” as part of the template
- Bugs will be triaged weekly (this could be modified based on incoming bugs rate), and upon triage will be assigned a priority or closed.
- All of priority/important-soon and some of priority/important-longterm bugs will be planned for next release cycle to minimize engineering debt.
- priority/critical-urgent is reserved for critical bugs that needs to be addressed immediately in current release at the cost of currently planned items.
- Bugs that need more information to decide priority will be labeled priority/awaiting-more-evidence
- WG lead will drive this and their decision will be final

## How to manage Story/Feature?
- Story/Feature typically spans 1-2 weeks for one engineer.
- To propose a new feature,  create a new issue of type “Feature request”
- Please fill in as much detail as possible in the Feature template and propose a release cycle
- All new features will be labelled “kind/feature-request”
- All new proposals are discussed either in WG meeting, or on slack, or dedicated meetings if needed at any point scheduled by the proposer. 
- By **R-2w’s** WG meeting all new proposals should be vetted and verified that they fit the [Planning criteria](#planning-criteria). Features that do not fit this criteria will not be considered in the current release planning cycle and hence early discussion is encouraged instead of waiting for **R-2w**.
- All eligible proposals will then be snapshotted into the voting document and open for voting till **R-1w** WG meeting. In **R-1w** WG meeting top features based on votes will be planned for next release.
- Accepted features will be added to the correct milestone and sprint specific project for tracking.

![planning overview](images/featureplanning.png)

> Example for 0.6 release.
  >- March 20, 2019 (Wednesday): All features and issues that fit the planning criteria are fed into voting document.
  >- March 27, 2019 (Wednesday): Pick top voted features and add them to 0.6 release based on estimates.
  >- April 2, 2019 (Tuesday): 0.5 release to market. Knative releases are planned every 6th Tuesday.



> In future, we will need some kind of estimates for each feature for release planning. Based on team feedback, we will get to this if needed in future.


## How to manage Epic/Scenario?
- Epic/Scenario typically spans 1-3 months or even more in some cases.
- For new proposals these will be created and triaged/refined just like Story/Feature. However, after triaging these need to be broken down into Features for planning and voting.
- Github doesn’t allow nested Issues, hence the only good way to track these are by creating Projects and adding the broken down features as issues to this project. An issue can be part of multiple projects and hence we can get a Scenario\Epic view as well as a Sprint view through respective projects.

## Planning criteria:
- Features are well defined and scoped with clear exit criteria. No feature creeps once release starts
- Features can be implemented in <2w by one engineer. In case of Epic/Scenario, it could be broken down into <2w features with clear exit criteria.
- The engineering team should be confident that any engineer can implement the feature based on the content in the feature *(This is similar to refinement in a scrum, but doesn’t have to be as strict as a functional specification)*.

## Organizing with Github Projects 
- New Project per sprint. Today, the sprint and milestone are both aligned. However in future we may decide to have multiple sprints in a release cycle.
- New Project per any Story/Scenario
- Any other projects that help organize and will be created when needed.

## Organizing with Github Milestone 
- A new milestone is created in Github for each 6 weeks release cycle.
- All features and bugs that are planned for the release cycle are assigned to the milestone for tracking the progress of the release.

> - Based on team feedback we will use GitHub milestone to track release progress. We will explore project boards and see if it can be automated with no overhead for engineers and then decide whether to use them or not.
> - However projects will be used for Epics

## To explore for improvements in future:
- Decide estimation technique (story points on fibonacci, t-shirt size etc) - We will get to this later if the team feels it is needed.
- How to automate project boards with Prow and issue labels?
- If the above process works for next couple of releases and team likes it then we move to a 2 week sprint model rather than 6 weeks.
