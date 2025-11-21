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

# Track failures
FAILED=0

###########################################
# 1. Code Formatting Check (gofmt)
###########################################
echo -e "${BLUE}[1/5] Checking code formatting with gofmt...${NC}"
UNFORMATTED=$(gofmt -s -l . 2>&1)
if [ -n "$UNFORMATTED" ]; then
    echo -e "${RED}✗ The following files are not formatted:${NC}"
    echo "$UNFORMATTED"
    echo -e "${YELLOW}Run: gofmt -s -w .${NC}"
    FAILED=$((FAILED + 1))
else
    echo -e "${GREEN}✓ All files are properly formatted${NC}"
fi
echo ""

###########################################
# 2. Static Analysis (go vet)
###########################################
echo -e "${BLUE}[2/5] Running static analysis with go vet...${NC}"
if go vet ./... 2>&1; then
    echo -e "${GREEN}✓ go vet passed${NC}"
else
    echo -e "${RED}✗ go vet found issues${NC}"
    FAILED=$((FAILED + 1))
fi
echo ""

###########################################
# 3. Tests with Race Detection
###########################################
echo -e "${BLUE}[3/5] Running tests with race detection...${NC}"
if go test -v -race -coverprofile=coverage.out -covermode=atomic ./... 2>&1; then
    echo -e "${GREEN}✓ All tests passed${NC}"
else
    echo -e "${RED}✗ Tests failed${NC}"
    FAILED=$((FAILED + 1))
fi
echo ""

###########################################
# 5. Coverage Report (informational only)
###########################################
echo -e "${BLUE}[4/5] Reporting test coverage (informational)...${NC}"
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
# 6. Build Check
###########################################
echo -e "${BLUE}[5/5] Building binary...${NC}"
if go build -v -o ghcrctl . 2>&1 > /dev/null; then
    echo -e "${GREEN}✓ Build successful${NC}"

    # Verify binary works
    if ./ghcrctl --help > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Binary verification successful${NC}"
    else
        echo -e "${RED}✗ Binary verification failed${NC}"
        FAILED=$((FAILED + 1))
    fi

    # Clean up
    rm -f ghcrctl
else
    echo -e "${RED}✗ Build failed${NC}"
    FAILED=$((FAILED + 1))
fi
echo ""

###########################################
# Summary
###########################################
echo -e "${BLUE}================================================${NC}"
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All checks passed! Ready to commit.${NC}"
    echo -e "${BLUE}================================================${NC}"
    exit 0
else
    echo -e "${RED}✗ ${FAILED} check(s) failed. Please fix the issues above.${NC}"
    echo -e "${BLUE}================================================${NC}"
    exit 1
fi
