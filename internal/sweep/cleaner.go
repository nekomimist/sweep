package sweep

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
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
	SkippedKept   int
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

type cleanupCandidate struct {
	path       string
	modTime    time.Time
	emacsKey   string
	generation int
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
		fmt.Fprintf(c.out, "Keep Emacs Backups: %d\n", c.config.KeepEmacsBackups)
	}

	if c.config.Confirm && !c.config.DryRun {
		message := fmt.Sprintf("Are you sure you want to delete backup files in %s? (y/n): ", c.config.Directory)
		if !c.askConfirmation(message) {
			fmt.Fprintln(c.out, "Operation cancelled.")
			return result
		}
	}

	var candidates []cleanupCandidate
	err := c.fs.WalkDir(c.config.Directory, func(path string, entry fs.DirEntry, walkErr error) error {
		return c.walkFunc(&result, &candidates, path, entry, walkErr)
	})
	result.addError(err)
	protected := c.protectedEmacsBackups(candidates)
	for _, candidate := range candidates {
		c.processCandidate(&result, candidate, protected)
	}
	return result
}

func (c *Cleaner) walkFunc(result *Result, candidates *[]cleanupCandidate, path string, entry fs.DirEntry, walkErr error) error {
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

	candidate := cleanupCandidate{
		path:    absPath,
		modTime: fileInfo.ModTime(),
	}
	if key, generation, ok := emacsNumberedBackup(absPath); ok {
		candidate.emacsKey = key
		candidate.generation = generation
	}
	*candidates = append(*candidates, candidate)
	return nil
}

func (c *Cleaner) processCandidate(result *Result, candidate cleanupCandidate, protected map[string]struct{}) {
	age := time.Since(candidate.modTime)
	if _, ok := protected[candidate.path]; ok {
		result.SkippedKept++
		c.verboseLog("Skip: %s is a kept Emacs numbered backup generation\n", candidate.path)
		return
	}
	if age < c.config.MinAge {
		result.SkippedNew++
		c.verboseLog("Skip: %s is too new (age: %s, min: %s)\n",
			candidate.path, age.Round(time.Hour), c.config.MinAge.Round(time.Hour))
		return
	}

	c.handleFile(result, candidate.path, age)
}

func shouldDelete(path string) bool {
	return strings.HasSuffix(path, "~") || strings.HasSuffix(path, ".bak")
}

func emacsNumberedBackup(path string) (string, int, bool) {
	base := filepath.Base(path)
	idx := strings.LastIndex(base, ".~")
	if idx < 0 || !strings.HasSuffix(base, "~") {
		return "", 0, false
	}

	digits := base[idx+2 : len(base)-1]
	if digits == "" {
		return "", 0, false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return "", 0, false
		}
	}

	generation, err := strconv.Atoi(digits)
	if err != nil {
		return "", 0, false
	}
	return path[:len(path)-len(base)+idx], generation, true
}

func (c *Cleaner) protectedEmacsBackups(candidates []cleanupCandidate) map[string]struct{} {
	protected := map[string]struct{}{}
	if c.config.KeepEmacsBackups == 0 {
		return protected
	}

	groups := map[string][]cleanupCandidate{}
	for _, candidate := range candidates {
		if candidate.emacsKey == "" {
			continue
		}
		groups[candidate.emacsKey] = append(groups[candidate.emacsKey], candidate)
	}

	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			return group[i].generation > group[j].generation
		})
		limit := c.config.KeepEmacsBackups
		if len(group) < limit {
			limit = len(group)
		}
		for _, candidate := range group[:limit] {
			protected[candidate.path] = struct{}{}
		}
	}

	return protected
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
