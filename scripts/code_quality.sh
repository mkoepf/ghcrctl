#!/usr/bin/env bash
#
# check.sh - Run all code quality checks from CI pipeline locally
#
# This script runs the same checks that are executed in the GitHub Actions CI workflow.
# Run this before committing to catch issues early.

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COVERAGE_THRESHOLD=60

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}Running Code Quality Checks${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

###########################################
# 1. Code Formatting Check (gofmt)
###########################################
echo -e "${BLUE}[1/7] Checking code formatting with gofmt...${NC}"
UNFORMATTED=$(gofmt -s -l . 2>&1)
if [ -n "$UNFORMATTED" ]; then
    echo -e "${RED}✗ The following files are not formatted:${NC}"
    echo "$UNFORMATTED"
    echo -e "${YELLOW}Run: gofmt -s -w .${NC}"
    exit 1
else
    echo -e "${GREEN}✓ All files are properly formatted${NC}"
fi
echo ""


###########################################
# 2. Build Check
###########################################
echo -e "${BLUE}[2/7] Building binary...${NC}"
if go build -v -o ghcrctl . 2>&1 > /dev/null; then
    echo -e "${GREEN}✓ Build successful${NC}"

    # Verify binary works
    if ./ghcrctl --help > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Binary verification successful${NC}"
    else
        echo -e "${RED}✗ Binary verification failed${NC}"
        exit 1
    fi

    # Clean up
    rm -f ghcrctl
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi
echo ""


###########################################
# 3. Static Analysis (go vet)
###########################################
echo -e "${BLUE}[3/7] Running static analysis with go vet...${NC}"
if go vet ./... 2>&1; then
    echo -e "${GREEN}✓ go vet passed${NC}"
else
    echo -e "${RED}✗ go vet found issues${NC}"
    exit 1
fi
echo ""


###########################################
# 4. Tests with Race Detection and SKIP detection
###########################################
echo -e "${BLUE}[4/7] Running readonly tests with race and skip detection...${NC}"
# Run go test and capture combined stdout+stderr
output=$(go test -json -v -race \
    -coverprofile=coverage.out \
    -covermode=atomic \
    -coverpkg=./... \
    ./... 2>&1)

test_exit_code=$?   # exit code of go test

# Fail if any tests failed
if [ "$test_exit_code" -ne 0 ]; then
    echo -e "${RED}✗ One or more tests failed${NC}"
    exit 1
fi

# Fail if any tests were skipped
if echo "$output" | grep -q '"Action":"skip"'; then
    echo -e "${RED}✗ One or more tests were SKIPPED${NC}"
    exit 1
fi

echo -e "${GREEN}✓ All tests passed${NC}"

echo ""

###########################################
# 5. Mutating Tests (without race detection)
###########################################
echo -e "${BLUE}[5/7] Running mutating tests with skip detection...${NC}"
# Run mutating tests without -race flag due to race conditions in oras-go library's
# HTTP/2 handling during push operations. This only affects test setup code (CopyImage),
# not the actual functional code being tested.
mutating_output=$(go test -json -v \
    -tags=mutating \
    -coverprofile=coverage-mutating.out \
    -covermode=atomic \
    -coverpkg=./... \
    ./... 2>&1)

mutating_exit_code=$?   # exit code of go test

# Fail if any tests failed
if [ "$mutating_exit_code" -ne 0 ]; then
    echo -e "${RED}✗ One or more mutating tests failed${NC}"
    exit 1
fi

# Fail if any tests were skipped
if echo "$mutating_output" | grep -q '"Action":"skip"'; then
    echo -e "${RED}✗ One or more mutating tests were SKIPPED${NC}"
    exit 1
fi

echo -e "${GREEN}✓ All mutating tests passed${NC}"

# Merge coverage files
if [ -f coverage.out ] && [ -f coverage-mutating.out ]; then
    echo -e "${BLUE}Merging coverage data...${NC}"
    GOCOVMERGE="$(go env GOPATH)/bin/gocovmerge"
    # Install gocovmerge if not present
    if [ ! -f "$GOCOVMERGE" ]; then
        go install github.com/wadey/gocovmerge@latest
    fi
    "$GOCOVMERGE" coverage.out coverage-mutating.out > coverage-merged.out
    mv coverage-merged.out coverage.out
    rm -f coverage-mutating.out
    echo -e "${GREEN}✓ Coverage data merged${NC}"
fi

echo ""

###########################################
# 6. Security scans
###########################################
echo -e "${BLUE}[6/7] Running security scans... ${NC}"
govulncheck ./...
gosec ./...
trivy fs . --skip-dirs .claude --scanners=vuln,misconfig,secret --exit-code 1

###########################################
# 7. Coverage Report (informational only)
###########################################
echo -e "${BLUE}[7/7] Reporting test coverage (informational)...${NC}"
if [ -f coverage.out ]; then
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    echo -e "Total coverage: ${YELLOW}${COVERAGE}%${NC}"
    echo -e "${BLUE}Note: Coverage is measured but does not fail checks${NC}"

    # Show coverage by package
    echo ""
    echo -e "${BLUE}Coverage by package:${NC}"
    go tool cover -func=coverage.out | grep -v "total:" | while read -r line; do
        if echo "$line" | grep -q "100.0%"; then
            echo -e "${GREEN}${line}${NC}"
        elif echo "$line" | awk '{if ($NF+0 >= 80) exit 0; else exit 1}' 2>/dev/null; then
            echo -e "${YELLOW}${line}${NC}"
        else
            echo -e "${line}"
        fi
    done
else
    echo -e "${YELLOW}⚠ No coverage file found${NC}"
fi
echo ""

###########################################
# Summary
###########################################
echo -e "${BLUE}================================================${NC}"
echo -e "${GREEN}✓ All checks passed! Ready to commit.${NC}"
echo -e "${BLUE}================================================${NC}"
exit 0
