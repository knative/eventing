# Duty

Every week we dedicate one individual (the community contact) to watch for user
issues and either answer them or redirect the questions to other who can. The
community contact's duty (subject to change) is as followed:

## Daily check list

- Check the knative-users@ mailing list and #eventing-questions Slack channel
  for new unanswered questions.
- Check Eventing
  [unassigned issues / untriaged issues](https://github.com/knative/eventing/issues?q=is%3Aopen+is%3Aissue+no%3Aassignee+no%3Amilestone)
  for unanswered questions.
- Check Slack channel
  [#eventing-questions](https://knative.slack.com/archives/C017X0PFC0P) for
  unanswered questions. Any questions that relates to usability please instruct
  user to
  [open an usablity issue](https://github.com/knative/ux/issues/new?assignees=&labels=kind%2Ffriction-point&template=friction-point-template.md&title=)
  and to join the channel
  [#user-experience](https://knative.slack.com/archives/C01JBD1LSF3) to capture
  user feedback.
- Check Docs
  [unassigned issues / untriaged issues](https://github.com/knative/docs/issues?q=is%3Aopen+is%3Aissue+label%3Akind%2Feventing+label%3Atriage%2Fneeds-eng-input)
  for unanswered questions.
- You should already be added to @eventing-help usergroup by the previous
  contact person, and will be tagged into user questions in other channels.
- Answer relevant
  [Stack Overflow Questions](https://stackoverflow.com/questions/tagged/knative-eventing?tab=Newest)

## Weekly check list

### Monday

- Check 'Eventing Triage' Peribolos group
  https://github.com/knative/community/blob/main/peribolos/knative.yaml and send
  a PR to add yourself if you aren't already in there.
- Check the [Eventing test grid](https://testgrid.k8s.io/knative-eventing) for
  flakiness to pick a test and focus on fixing it during your week. Once you
  pick the test flake, assign the corresponding bug filed by flakiness test
  reporter to yourself so that others won't pick the same test to fix.
- Join the @eventing-help user group if you haven't been invited by the previous
  contact person using this
  [link](https://app.slack.com/client/T93ELUK42/browse-user-groups/user_groups/S0191HUK2Q0)

### Tuesday - Thursday

Continue on fixing test flakes and answer

### Friday

- Remove yourself from @eventing-help usergroup and add the next contact using
  this
  [link](https://app.slack.com/client/T93ELUK42/browse-user-groups/user_groups/S0191HUK2Q0).
  If you don't have permission, ask in the Slack channel
  #steering-toc-questions.
- Email the next contacts, cc'ing knative-dev@ with a short summaries of the
  user questions encountered and links to them.
- File issue or document suggested improvements to this process.

## SLO

Participation is voluntary and based on good faith. The community contacts are
only expected to participate during their local office hour.

# Roster

We seed this rotation with all approvers and reviewers from all the Eventing
workgroups, excluding productivity. If you are no longer active in Knative, or
if you are contributing on personal capacity and do not have time to contribute
in the rotation, feel free to send a PR to remove yourself and add yourself to
the NON_ROSTER file. Similarly you can trade your timeslot with others by
sending PRs.

## Volunteeres

Those who aren't reviewers or approvers and are interested in volunteering are
welcome to add themselves to the `support/VOLUNTEERS` file.

## Generating a New Schedule

The schedule can be regenerated randomly for a new cycle with the command (run
from the eventing base directory).

```sh
VOLUNTEERS=$(grep -hs "" support/VOLUNTEERS)
OWNERS=$(grep "^- " OWNERS | cut -f2 -d" ")
NON_ROSTER=$(grep -v -e '^$' support/NON_ROSTER)
echo ${VOLUNTEERS} ${OWNERS} | sed 's/ /\n/g' | grep -v ${NON_ROSTER} | sort | uniq | sort -R
```

- [antoineco](https://github.com/antoineco)
- [lionelvillard](https://github.com/lionelvillard)
- [matzew](https://github.com/matzew)
- [n3wscott](https://github.com/n3wscott)
- [slinkydeveloper](https://github.com/slinkydeveloper)
- [vaikas](https://github.com/vaikas)
- [lberk](https://github.com/lberk)
- [pierDipi](https://github.com/pierDipi)
- [aliok](https://github.com/aliok)
- [aslom](https://github.com/aslom)
- [tayarani](https://github.com/tayarani)
- [zhongduo](https://github.com/zhongduo)
- [nlopezgi](https://github.com/nlopezgi)
- [devguyio](https://github.com/devguyio)

# Schedule

See [a machine-readable schedule here](support.rotation). The format is:

```
# comment lines are okay
#@ metadata: value of the metadata
RFC3339-date  |  username
```

You can see the current oncall at https://knative.party/ (which reads the
machine-readable file).
