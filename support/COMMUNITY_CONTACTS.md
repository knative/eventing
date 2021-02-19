# Duty

Every week we dedicate one individual (the community contact) to watch for user
issues and either answer them or redirect the questions to other who can. The
community contact's duty (subject to change) is as followed:

## Daily check list

- Check the knative-users@ mailing list and #eventing-questions Slack channel
  for new unanswered questions.
- Check
  [unassigned issues / untriaged issues](https://github.com/knative/eventing/issues?q=is%3Aopen+is%3Aissue+no%3Aassignee+no%3Amilestone)
  for unanswered questions.
- You should already be added to @eventing-help usergroup by the previous
  contact person, and will be tagged into user questions in other channels.
- Answer relevant
  [Stack Overflow Questions](https://stackoverflow.com/questions/tagged/knative-eventing?tab=Newest)

## Weekly check list

### Monday

- Check 'Eventing Triage' Peribolos group
  https://github.com/knative/community/blob/master/peribolos/knative.yaml and
  send a PR to add yourself if you aren't already in there.
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

- [grantr](https://github.com/grantr)
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
- [tommyreddad](https://github.com/tommyreddad)
- [nlopezgi](https://github.com/nlopezgi)
- [devguyio](https://github.com/devguyio)

# Schedule

| Week       | Contact                                               |
| ---------- | ----------------------------------------------------- |
| 2021-01-04 | [n3wscott](https://github.com/n3wscott)               |
| 2021-01-11 | [vaikas](https://github.com/vaikas)                   |
| 2021-01-18 | [grantr](https://github.com/grantr)                   |
| 2021-01-25 | [lionelvillard](https://github.com/lionelvillard)     |
| 2021-02-01 | [zhongduo](https://github.com/zhongduo)               |
| 2021-02-08 | [antoineco](https://github.com/antoineco)             |
| 2021-02-15 | [matzew](https://github.com/matzew)                   |
| 2021-02-22 | [vaikas](https://github.com/vaikas)                   |
| 2021-03-01 | [pierDipi](https://github.com/pierDipi)               |
| 2021-03-08 | [aliok](https://github.com/aliok)                     |
| 2021-03-15 | [slinkydeveloper](https://github.com/slinkydeveloper) |
| 2021-03-22 | [devguyio](https://github.com/devguyio)               |
| 2021-03-29 | [n3wscott](https://github.com/n3wscott)               |
| 2021-04-05 | [lberk](https://github.com/lberk)                     |
| 2021-04-12 | [grantr](https://github.com/grantr)                   |
| 2021-04-19 | [lionelvillard](https://github.com/lionelvillard)     |
| 2021-04-26 | [slinkydeveloper](https://github.com/slinkydeveloper) |
| 2021-05-03 | [antoineco](https://github.com/antoineco)             |
| 2021-05-10 | [matzew](https://github.com/matzew)                   |
| 2021-05-17 | [lberk](https://github.com/lberk)                     |
| 2021-05-24 | [pierDipi](https://github.com/pierDipi)               |
| 2021-05-31 | [aliok](https://github.com/aliok)                     |
| 2021-06-07 | [zhongduo](https://github.com/zhongduo)               |
| 2021-06-14 | [devguyio](https://github.com/devguyio)               |
