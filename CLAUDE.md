# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Workflow (CRITICAL)

**STRICT TDD workflow required:**
1. Ask: How can this be tested?
2. Implement the test
3. Run test to confirm it fails
4. Implement code to make test pass
5. Confirm test passes
6. Confirm ALL tests pass
7. Commit with descriptive message

**NEVER write non-test code before there is a corresponding test.**

Progress SLOWLY and iteratively (max 10 changed lines at a time).