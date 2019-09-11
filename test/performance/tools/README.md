# Tools

This directory contains all the files needed to manipulate all the benchmarking
clusters. It contains of

- common.sh: contains common functions that can be used in other scripts.

- recreate_clusters.sh: This will:

  - Read all the current clusters in the `knative-eventing-performance` project
  - Kill all the current K8S objects
  - Delete the existing cluster
  - Recreate the cluster with the same name, node-count and in the same zone
  - Install knative eventing
  - Apply patches(if any) to setup performance testing
  - Run `ko apply` to all objects in the test-dir. Note that, this assuumes that
    the dir name and cluster name are the same.

- update_clusters.sh: This will:

  - Read all the current clusters in the `knative-eventing-performance` project
  - Kill all the current K8S objects
  - Install knative eventing
  - Apply patches(if any) to setup performance testing
  - Run `ko apply` to all objects in the test-dir. Note that, this assuumes that
    the dir name and cluster name are the same.
