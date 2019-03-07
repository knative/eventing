# Broker and Trigger model (for 0.5.0)

Decision date: 6 March 2019

##

## Backing documents:

[2019-02-22 Decision Making -- UX targets](https://docs.google.com/spreadsheets/d/16aOhfRnkaGcQIOR5kiumld-GmrgGBIm9fppvAXx3mgc/edit)

Accepted decisions (2+ votes)

- Have a "Broker" object
  - Default sink on sources
  - Enable with namespace/default Broker
- Have a "Trigger" object
  - "filter" subfield (not top-level filters)
  - "filter" has exact type match
  - "filter" has exact source match
  - target is same-namespace

[UX Proposals Comparision](https://docs.google.com/document/d/1fRpM4u4mP2fGUBmScKQ9_e77rKz_7xh_Thwxp8QXhUA/edit#)
(includes background and discussion)
