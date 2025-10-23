#!/bin/bash
set -e

# This script generates golden RSS files for testing
# Run this when you intentionally change RSS output format

echo "Generating test fixtures..."

# Build the binary
go build -o bookast

# Generate RSS for test audiobook
./bookast --base-url https://example.com/audiobooks testdata/audiobook1

# Move the generated RSS to golden file
mv testdata/audiobook1/podcast.rss testdata/audiobook1/golden.rss

echo "Golden file created: testdata/audiobook1/golden.rss"
echo "Review the file to ensure it's correct, then commit it."

# Clean up binary
rm bookast
