#!/bin/bash

if [ "$1" == "" ] || [ $# -gt 1 ]; then
    echo "Error: Expecting an argument" >&2
    echo "usage: $(basename $0) revision_to_compare" >&2
    exit 1
fi

go get golang.org/x/perf/cmd/benchstat

REVISION="$1"
OUTPUT_DIR=${ARTIFACTS:-$(mktemp -d)}

./microbenchmarks-run.sh "$OUTPUT_DIR/new.txt"

git checkout "$REVISION"

./microbenchmarks-run.sh "$OUTPUT_DIR/old.txt"

benchstat -html "$OUTPUT_DIR/old.txt" "$OUTPUT_DIR/new.txt" > "OUTPUT_DIR/results.html"
