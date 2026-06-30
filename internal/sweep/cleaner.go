package sweep

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// Result summarizes a cleanup run.
type Result struct {
	Checked       int
	Matched       int
	Removed       int
	WouldRemove   int
	SkippedNew    int
	SkippedDenied int
	Errors        []error
}

func (r *Result) addError(err error) {
	if err != nil {
		r.Errors = append(r.Errors, err)
	}
}

func (r Result) Err() error {
	if len(r.Errors) == 0 {
		return nil
	}
	return errors.Join(r.Errors...)
}

// Cleaner handles file cleanup.
type Cleaner struct {
	fs     FileSystem
	config Config
	in     *bufio.Reader
	out    io.Writer
	errOut io.Writer
}

// NewCleaner creates a Cleaner.
func NewCleaner(fileSystem FileSystem, config Config, input io.Reader, output io.Writer, errorOutput io.Writer) *Cleaner {
	return &Cleaner{
		fs:     fileSystem,
		config: config,
		in:     bufio.NewReader(input),
		out:    output,
		errOut: errorOutput,
	}
}

// Clean performs the cleanup operation.
func (c *Cleaner) Clean() Result {
	var result Result

	if c.config.Verbose {
		fmt.Fprintf(c.out, "Exclude Pattern: %s\n", c.config.ExcludePattern)
		fmt.Fprintf(c.out, "Target Directory: %s\n", c.config.Directory)
		fmt.Fprintf(c.out, "Minimum Age: %s\n", c.config.MinAge)
	}

	if c.config.Confirm && !c.config.DryRun {
		message := fmt.Sprintf("Are you sure you want to delete backup files in %s? (y/n): ", c.config.Directory)
		if !c.askConfirmation(message) {
			fmt.Fprintln(c.out, "Operation cancelled.")
			return result
		}
	}

	err := c.fs.WalkDir(c.config.Directory, func(path string, entry fs.DirEntry, walkErr error) error {
		return c.walkFunc(&result, path, entry, walkErr)
	})
	result.addError(err)
	return result
}

func (c *Cleaner) walkFunc(result *Result, path string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		result.addError(fmt.Errorf("skip %s: %w", path, walkErr))
		fmt.Fprintf(c.errOut, "Error: skip %s\n", path)
		if entry != nil && entry.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	if entry == nil {
		result.addError(fmt.Errorf("skip %s: missing directory entry", path))
		fmt.Fprintf(c.errOut, "Error: skip %s\n", path)
		return nil
	}

	absPath, err := c.fs.Abs(path)
	if err != nil {
		result.addError(fmt.Errorf("cannot get absolute path for %s: %w", path, err))
		fmt.Fprintf(c.errOut, "Error: cannot get absolute path for %s\n", path)
		return nil
	}

	excludeMatched := c.config.ExcludeRegexp.MatchString(absPath)
	if excludeMatched && entry.IsDir() {
		return filepath.SkipDir
	}
	if excludeMatched || !entry.Type().IsRegular() {
		return nil
	}

	result.Checked++
	c.verboseLog("Check1: %s\n", absPath)
	if !shouldDelete(absPath) {
		return nil
	}

	result.Matched++
	c.verboseLog("Check2: %s\n", absPath)

	fileInfo, err := entry.Info()
	if err != nil {
		result.addError(fmt.Errorf("cannot get file info for %s: %w", absPath, err))
		fmt.Fprintf(c.errOut, "Error: cannot get file info for %s\n", absPath)
		return nil
	}

	age := time.Since(fileInfo.ModTime())
	if age < c.config.MinAge {
		result.SkippedNew++
		c.verboseLog("Skip: %s is too new (age: %s, min: %s)\n",
			absPath, age.Round(time.Hour), c.config.MinAge.Round(time.Hour))
		return nil
	}

	c.handleFile(result, absPath, age)
	return nil
}

func shouldDelete(path string) bool {
	return strings.HasSuffix(path, "~") || strings.HasSuffix(path, ".bak")
}

func (c *Cleaner) handleFile(result *Result, path string, age time.Duration) {
	if c.config.DryRun {
		result.WouldRemove++
		fmt.Fprintf(c.out, "Would remove: %s (age: %s)\n", path, age.Round(time.Hour))
		return
	}

	if c.config.Interactive && !c.askConfirmation(fmt.Sprintf("Delete %s? (y/n): ", path)) {
		result.SkippedDenied++
		return
	}

	if err := c.fs.Remove(path); err != nil {
		result.addError(fmt.Errorf("cannot remove %s: %w", path, err))
		fmt.Fprintf(c.errOut, "Error: cannot remove %s\n", path)
		return
	}

	result.Removed++
	fmt.Fprintf(c.out, "Removed: %s\n", path)
}

func (c *Cleaner) askConfirmation(message string) bool {
	fmt.Fprint(c.out, message)
	response, err := c.in.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func (c *Cleaner) verboseLog(format string, args ...interface{}) {
	if c.config.Verbose {
		fmt.Fprintf(c.out, format, args...)
	}
}
