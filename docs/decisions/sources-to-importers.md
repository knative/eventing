# Renaming Sources to Importers

Decision date: 4 June 2019

## What

Rename Eventing's existing Source concept to avoid conflict with the subtly
different CloudEvents concept of source. The name Importer was chosen to imply
that events emitted from an Importer were imported from an external system and
did not originate at the Importer.

Some sources such as CronSource do not technically import events from outside
the cluster, but may still reasonably be called Importers because they are not
themselves the originator of the event. This may be revisited in the future.

## Details

For 0.7 we will note the name change as an FYI in docs to start training users
already familiar with the Source concept.

For 0.8 we will rename Source to Importer in docs (with a note that Importers
used to be called Sources) and rename existing `*Source` objects to Importer.
Further details TBD.

## Source documents

https://github.com/knative/eventing-sources/issues/402
https://github.com/knative/eventing/issues/1344
