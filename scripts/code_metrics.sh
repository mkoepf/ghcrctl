#!/bin/bash
#
# Code Metrics Script for ghcrctl
# Measures lines of code by component, distinguishing test code from production code
#

set -e

# Colors for output
BOLD='\033[1m'
RESET='\033[0m'
GRAY='\033[90m'

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# Function to count lines in files (excluding blank lines and comments)
count_lines() {
    local files="$1"
    if [ -z "$files" ]; then
        echo 0
        return
    fi
    # Count non-blank, non-comment lines
    echo "$files" | xargs grep -v '^\s*$' 2>/dev/null | grep -v '^\s*//' | wc -l | tr -d ' '
}

# Function to count raw lines (including blanks and comments)
count_raw_lines() {
    local files="$1"
    if [ -z "$files" ]; then
        echo 0
        return
    fi
    echo "$files" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}'
}

# Function to count files
count_files() {
    local files="$1"
    if [ -z "$files" ]; then
        echo 0
        return
    fi
    echo "$files" | wc -l | tr -d ' '
}

# Print header
echo ""
echo -e "${BOLD}Code Metrics Report${RESET}"
echo -e "${GRAY}Generated: $(date '+%Y-%m-%d %H:%M:%S')${RESET}"
echo -e "${GRAY}Project: ghcrctl${RESET}"
echo ""

# Define components
declare -a COMPONENTS=(
    "cmd:cmd"
    "internal/gh:internal/gh"
    "internal/oras:internal/oras"
    "internal/discovery:internal/discovery"
    "internal/discover:internal/discover"
    "internal/display:internal/display"
    "internal/filter:internal/filter"
    "internal/logging:internal/logging"
    "internal/prompts:internal/prompts"
    "root:."
)

# Initialize totals
TOTAL_PROD_FILES=0
TOTAL_PROD_LINES=0
TOTAL_TEST_FILES=0
TOTAL_TEST_LINES=0

# Print table header
printf "${BOLD}%-25s %8s %10s %8s %10s %10s${RESET}\n" \
    "COMPONENT" "FILES" "LINES" "TESTS" "TEST LINES" "TOTAL"
printf "%-25s %8s %10s %8s %10s %10s\n" \
    "-------------------------" "--------" "----------" "--------" "----------" "----------"

# Process each component
for component in "${COMPONENTS[@]}"; do
    IFS=':' read -r name path <<< "$component"

    if [ "$path" = "." ]; then
        # Root level - only direct files, no subdirectories
        PROD_FILES=$(find "$path" -maxdepth 1 -name "*.go" ! -name "*_test.go" -type f 2>/dev/null | sort)
        TEST_FILES=$(find "$path" -maxdepth 1 -name "*_test.go" -type f 2>/dev/null | sort)
    else
        # Component directory
        if [ ! -d "$path" ]; then
            continue
        fi
        PROD_FILES=$(find "$path" -name "*.go" ! -name "*_test.go" -type f 2>/dev/null | sort)
        TEST_FILES=$(find "$path" -name "*_test.go" -type f 2>/dev/null | sort)
    fi

    # Count metrics
    PROD_FILE_COUNT=$(count_files "$PROD_FILES")
    TEST_FILE_COUNT=$(count_files "$TEST_FILES")

    if [ "$PROD_FILE_COUNT" -gt 0 ]; then
        PROD_LINE_COUNT=$(count_raw_lines "$PROD_FILES")
    else
        PROD_LINE_COUNT=0
    fi

    if [ "$TEST_FILE_COUNT" -gt 0 ]; then
        TEST_LINE_COUNT=$(count_raw_lines "$TEST_FILES")
    else
        TEST_LINE_COUNT=0
    fi

    COMPONENT_TOTAL=$((PROD_LINE_COUNT + TEST_LINE_COUNT))

    # Update totals
    TOTAL_PROD_FILES=$((TOTAL_PROD_FILES + PROD_FILE_COUNT))
    TOTAL_PROD_LINES=$((TOTAL_PROD_LINES + PROD_LINE_COUNT))
    TOTAL_TEST_FILES=$((TOTAL_TEST_FILES + TEST_FILE_COUNT))
    TOTAL_TEST_LINES=$((TOTAL_TEST_LINES + TEST_LINE_COUNT))

    # Print row
    printf "%-25s %8d %10d %8d %10d %10d\n" \
        "$name" "$PROD_FILE_COUNT" "$PROD_LINE_COUNT" "$TEST_FILE_COUNT" "$TEST_LINE_COUNT" "$COMPONENT_TOTAL"
done

# Print totals
printf "%-25s %8s %10s %8s %10s %10s\n" \
    "-------------------------" "--------" "----------" "--------" "----------" "----------"

GRAND_TOTAL=$((TOTAL_PROD_LINES + TOTAL_TEST_LINES))
TOTAL_FILES=$((TOTAL_PROD_FILES + TOTAL_TEST_FILES))

printf "${BOLD}%-25s %8d %10d %8d %10d %10d${RESET}\n" \
    "TOTAL" "$TOTAL_PROD_FILES" "$TOTAL_PROD_LINES" "$TOTAL_TEST_FILES" "$TOTAL_TEST_LINES" "$GRAND_TOTAL"

# Print summary
echo ""
echo -e "${BOLD}Summary${RESET}"
echo "  Total Go files:      $TOTAL_FILES"
echo "  Production code:     $TOTAL_PROD_LINES lines ($TOTAL_PROD_FILES files)"
echo "  Test code:           $TOTAL_TEST_LINES lines ($TOTAL_TEST_FILES files)"
echo "  Total lines:         $GRAND_TOTAL"
echo ""

# Calculate test ratio
if [ "$TOTAL_PROD_LINES" -gt 0 ]; then
    TEST_RATIO=$(echo "scale=2; $TOTAL_TEST_LINES * 100 / $TOTAL_PROD_LINES" | bc)
    echo "  Test/Prod ratio:     ${TEST_RATIO}%"
fi

echo ""

# Detailed file breakdown (optional, shown with -v flag)
if [ "$1" = "-v" ] || [ "$1" = "--verbose" ]; then
    echo -e "${BOLD}Detailed File Breakdown${RESET}"
    echo ""

    for component in "${COMPONENTS[@]}"; do
        IFS=':' read -r name path <<< "$component"

        if [ "$path" = "." ]; then
            FILES=$(find "$path" -maxdepth 1 -name "*.go" -type f 2>/dev/null | sort)
        else
            if [ ! -d "$path" ]; then
                continue
            fi
            FILES=$(find "$path" -name "*.go" -type f 2>/dev/null | sort)
        fi

        if [ -n "$FILES" ]; then
            echo -e "${BOLD}$name/${RESET}"
            echo "$FILES" | while read -r file; do
                if [ -n "$file" ]; then
                    lines=$(wc -l < "$file" | tr -d ' ')
                    basename=$(basename "$file")
                    if [[ "$basename" == *"_test.go" ]]; then
                        printf "  ${GRAY}%-40s %6d lines (test)${RESET}\n" "$basename" "$lines"
                    else
                        printf "  %-40s %6d lines\n" "$basename" "$lines"
                    fi
                fi
            done
            echo ""
        fi
    done
fi
