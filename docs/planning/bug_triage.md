# Knative Eventing Bug Triage and Management

This document summarizes how bugs will be triaged and managed in Knative
Eventing WG. This is a supplement to [Sprint planning](sprintplanning.md)
process.

## Goal

- Keep a low bug count. Bug count should not be consistently increasing release
  over release esp [priority/important-longterm](#Bug-priorities).
- Process shouldn't be taxing for a single person or the team.
- Maintain high product quality.

## Bug priorities

- priority/critical-urgent: Bugs that need to be fixed now, and possibly
  cherrypicked into the most recent release. May need patch-fix. Such bugs will
  be assigned to current release milestone or a new milestone created for a
  patch-fix.
- priority/important-soon: Bugs that need to be fixed in the next release cycle,
  and but do not need a patch-fix. Such bugs will be assigned to the next
  release milestone.
- priority/important-longterm: Bugs that need to be fixed, and will be fixed in
  next 2-3 releases. Such bugs will be assigned to a future release milestone.
- priority/awaiting-more-evidence: Bugs which might need to be fixed, but need
  more details first.

## How to manage bugs?

- Submit new bugs anytime by creating a new issue of type “Bug”.
- Please fill the bug template and optionally propose a priority.
- All new bugs will be labelled “kind/bug” as part of the template Bugs will be
  triaged weekly (this could be modified based on incoming bugs rate), and upon
  triage will be assigned a priority or closed.
- During each release/sprint planning, all priority/important-soon and some of
  priority/important-longterm bugs need to be accounted while committing to new
  features and improvements. These will be marked with appropriate release along
  with the planned features.
- Bugs marked with current release always take priority over feature work in a
  particular release. This will help maintain a balance between code quality,
  bug triage bar, and incoming feature rate.

## Bug Triage

- WG lead will form a bug triage team (TODO: Ville)
- There will be one bug triage champion each week to triage incoming bugs and
  assign them correct priority. This role will be rotated among engineers in the
  bug triage team.
- We will have first 3-4 bug triage meetings as a weekly team meeting. This
  meeting will help the entire team to come to an agreement and set a bar for
  each priority level. WG lead will drive this meeting.
- Guidelines for bug triage:
  - Any priority/critical-urgent bug will disrupt the current release cycle and
    hence need to be approved by WG lead.
  - All priority/important-soon bugs will be part of the next release cycle.
  - Number of bugs with priority/important-longterm need to be kept low and
    non-increasing, release over release. Here “long term” refers to next 2-3
    release cycles. If we will not work on it in next 2-3 release cycles, then
    we should close the bug upon triaging. [Remember the goal](#Goal).
  - Bugs with priority/critical-urgent and priority/important-soon bugs should
    have an assigned owner who has acknowledged the ownership of the bug.
  - TODO: Update these guidelines based on learning from triage meetings. Triage
    team will own and update this document and WG lead will approve the changes.
- In case of conflict, final decision will be taken by WG lead.
- WG lead will periodically monitor bug count and trend to make sure we meet our
  [goal](#Goal).

## To explore:

- Need to explore what tools could be used to track and see bug trends.
