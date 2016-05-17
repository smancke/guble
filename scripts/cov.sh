#!/bin/bash -e
# Run from directory above via ./scripts/cov.sh
# Require installation of: `github.com/wadey/gocovmerge`

source scripts/generate_coverage.sh

# If we have an arg, assume travis run and push to coveralls. Otherwise launch browser results
go tool cover -html=full_cov.out
rm -f full_cov.out
