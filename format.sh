#!/bin/bash

# format.sh

# Function to check if a file should be excluded
should_exclude() {
    local file=$1
    shift
    for exclude in "$@"; do
        if [[ $file == ./$exclude || $file == ./$exclude/* ]]; then
            return 0
        fi
    done
    return 1
}

# Store command-line arguments in an array
excludes=("$@")

# Array to hold files to format
declare -a files_to_format

# Get modified .go files from Git and add them to files_to_format unless they are excluded
while IFS= read -r file; do
    if ! should_exclude "$file" "${excludes[@]}"; then
        files_to_format+=("$file")
    fi
done < <(git diff --name-only --diff-filter=d HEAD | grep '\.go$')

# Format each file
for file in "${files_to_format[@]}"; do
    echo "Formatting $file"
    go fmt "$file"
    goimports -w "$file"
done
