#!/usr/bin/env bash

if [ "$1" == "" ] || [ $# -gt 1 ]; then
    echo "Error: Expecting an argument" >&2
    echo "usage: $(basename $0) revision_to_compare" >&2
    exit 1
fi

GO111MODULE=off go get golang.org/x/perf/cmd/benchstat

REVISION="$1"
OUTPUT_DIR=${ARTIFACTS:-$(mktemp -d)}

echo "Outputs will be at $OUTPUT_DIR"

$(dirname $0)/microbenchmarks-run.sh "$OUTPUT_DIR/new.txt"

git checkout "$REVISION"

$(dirname $0)/microbenchmarks-run.sh "$OUTPUT_DIR/old.txt"

benchstat -html "$OUTPUT_DIR/old.txt" "$OUTPUT_DIR/new.txt" >> "$OUTPUT_DIR/results.html"
