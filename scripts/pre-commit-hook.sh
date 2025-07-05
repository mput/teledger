#!/bin/bash

# Pre-commit hook for Go project with linting and formatting

set -e

echo "Running pre-commit checks..."

# Check if this is an initial commit
if git rev-parse --verify HEAD >/dev/null 2>&1
then
    against=HEAD
else
    # Initial commit: diff against an empty tree object
    against=$(git hash-object -t tree /dev/null)
fi

# Get list of staged Go files
staged_go_files=$(git diff --cached --name-only --diff-filter=ACM $against | grep '\.go$' || true)

if [ -n "$staged_go_files" ]; then
    echo "Checking Go files: $staged_go_files"
    
    # Check if staged files are properly formatted
    echo "Checking formatting on staged files..."
    format_issues=""
    for file in $staged_go_files; do
        if [ -f "$file" ]; then
            if ! gofumpt -d "$file" > /dev/null 2>&1; then
                format_issues="$format_issues $file"
            fi
        fi
    done
    
    if [ -n "$format_issues" ]; then
        echo "ERROR: The following files are not properly formatted:$format_issues"
        echo "Run 'make format' to fix formatting issues."
        exit 1
    fi
    
    # Run linting
    echo "Running linting..."
    if ! make lint; then
        echo "ERROR: Linting failed"
        exit 1
    fi
    
    echo "Pre-commit checks completed successfully!"
else
    echo "No Go files to check."
fi

exit 0