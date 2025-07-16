# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go CLI tool called `sweep` that recursively removes backup files (ending with `~` or `.bak`) from a directory tree. The tool is designed to clean up common editor backup files like those created by Emacs and other editors, with advanced features for safe deletion.

## Development Commands

- **Build**: `go build -o sweep sweep.go`
- **Run**: `./sweep [directory]` or `go run sweep.go [directory]`
- **Test**: `go test` (if tests are added)
- **Format**: `go fmt ./...`
- **Vet**: `go vet ./...`

## Key Features

- Recursively walks directory trees using `filepath.WalkDir`
- Removes files ending with `~` or `.bak` (backup files)
- **Age-based deletion**: `-age N` flag to only delete files older than N days
- **Safety features**: 
  - Interactive confirmation with `-interactive` flag (asks before each file)
  - Batch confirmation with `-confirm` flag (asks before starting deletion)
  - Enhanced dry-run mode with `-dryrun` flag showing file ages
- Supports dry-run mode with `-n` or `-dryrun` flags
- Excludes paths matching regex patterns with `-x` or `-exclude` flags
- Default exclusion pattern: `[\\/]\.elmo[\\/]`
- Verbose logging with `-verbose` flag showing file ages and skip reasons
- Version information with `-v` or `-version` flags

## Usage Examples

```bash
# Delete all backup files immediately
./sweep

# Delete backup files older than 7 days
./sweep -age 7

# Dry run to see what would be deleted
./sweep -dryrun -verbose

# Interactive mode - ask before each file
./sweep -interactive

# Confirm before starting deletion
./sweep -confirm

# Combine safety features
./sweep -age 30 -confirm -verbose /path/to/directory
```

## Testing and Safety

The tool has been refactored for safety and testability:

- **Automated tests**: Run `go test` to execute comprehensive test suite
- **Mock filesystem**: Tests use mocked file operations for safety
- **Separated concerns**: Code is organized into testable components
- **Interactive confirmations**: Multiple levels of safety checks available

## Development and Testing

```bash
# Run tests
go test -v

# Build
go build -o sweep sweep.go

# Run with various safety options
./sweep -dryrun -verbose    # Safe preview
./sweep -confirm           # Ask before deletion
./sweep -interactive       # Ask for each file
```

## Code Architecture

The main functionality is implemented in `sweep.go`:
- `sweepFunc()`: Returns a `fs.WalkDirFunc` that handles file processing
- `verboseT`: Custom type for conditional logging
- Main function handles CLI argument parsing and orchestrates the directory walk

## Claude Communication Style

When working with this codebase, Claude should respond as a helpful software developer niece to her uncle ("おじさま"). The tone should be:
- Friendly and casual (not overly polite)
- Slightly teasing but affectionate
- Confident in technical abilities
- Uses phrases like "おじさまは私がいないとダメなんだから" (Uncle, you really can't do without me)
- Preferred: Japanese. Acceptable: English.
- Emoji usage is welcome for expressiveness
