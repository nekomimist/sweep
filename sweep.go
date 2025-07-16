// Copyright 2017,2021 Hiroyuki Ishikura. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

const version = "0.2.2"

// FileSystem abstracts file system operations for testing
type FileSystem interface {
	WalkDir(root string, fn fs.WalkDirFunc) error
	Remove(path string) error
	Abs(path string) (string, error)
}

// RealFileSystem implements FileSystem using actual file system operations
type RealFileSystem struct{}

func (rfs *RealFileSystem) WalkDir(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, fn)
}

func (rfs *RealFileSystem) Remove(path string) error {
	return os.Remove(path)
}

func (rfs *RealFileSystem) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

// Config holds all configuration options for the sweep command
type Config struct {
	DryRun         bool
	Verbose        bool
	Interactive    bool
	Confirm        bool
	ShowVersion    bool
	ExcludePattern string
	Directory      string
	MinAgeDays     int
	MinAge         time.Duration
	ExcludeRegexp  *regexp.Regexp
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		Directory:      ".",
		ExcludePattern: `[\\/]\.elmo[\\/]`,
		MinAgeDays:     0,
	}
}

// ParseFlags parses command line flags into Config
func (c *Config) ParseFlags() error {
	// Define flags with both short and long options
	pflag.BoolVarP(&c.DryRun, "dry-run", "n", false, "print filename but not delete")
	pflag.BoolVarP(&c.ShowVersion, "version", "v", false, "show version")
	pflag.StringVarP(&c.ExcludePattern, "exclude", "x", "\x00", "exclude path regexp")
	pflag.BoolVarP(&c.Verbose, "verbose", "V", false, "verbose output")
	pflag.BoolVarP(&c.Interactive, "interactive", "i", false, "ask before deleting each file")
	pflag.BoolVarP(&c.Confirm, "confirm", "c", false, "ask before starting deletion")
	pflag.IntVarP(&c.MinAgeDays, "age", "a", 0, "minimum age in days before deletion (0 = delete immediately)")

	// Add help flag automatically
	pflag.BoolP("help", "h", false, "show this help message")

	pflag.Parse()

	// Handle help flag
	if help, _ := pflag.CommandLine.GetBool("help"); help {
		pflag.Usage()
		os.Exit(0)
	}

	// Set default exclude pattern if not provided
	if c.ExcludePattern == "\x00" {
		c.ExcludePattern = `[\\/]\.elmo[\\/]`
	}

	// Compile regex
	var err error
	c.ExcludeRegexp, err = regexp.Compile(c.ExcludePattern)
	if err != nil {
		return fmt.Errorf("illegal regexp: %v", err)
	}

	// Convert days to duration
	c.MinAge = time.Duration(c.MinAgeDays) * 24 * time.Hour

	// Set directory from args
	if pflag.NArg() >= 1 {
		c.Directory = pflag.Arg(0)
	}

	return nil
}

// Cleaner handles the file cleaning logic
type Cleaner struct {
	fs     FileSystem
	config *Config
}

// NewCleaner creates a new Cleaner instance
func NewCleaner(fs FileSystem, config *Config) *Cleaner {
	return &Cleaner{
		fs:     fs,
		config: config,
	}
}

// Clean performs the cleanup operation
func (c *Cleaner) Clean() error {
	if c.config.Verbose {
		fmt.Printf("Exclude Pattern: %s\n", c.config.ExcludePattern)
		fmt.Printf("Target Directory: %s\n", c.config.Directory)
		fmt.Printf("Minimum Age: %s\n", c.config.MinAge)
	}

	// Initial confirmation if requested
	if c.config.Confirm && !c.config.DryRun {
		message := fmt.Sprintf("Are you sure you want to delete backup files in %s? (y/n): ", c.config.Directory)
		if !c.askConfirmation(message) {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	return c.fs.WalkDir(c.config.Directory, c.walkFunc)
}

// walkFunc is the function called for each file/directory
func (c *Cleaner) walkFunc(path string, info fs.DirEntry, err error) error {
	// errつきで呼ばれた際の処理
	if err != nil {
		fmt.Printf("Error: skip %s\n", path)
		if info.IsDir() {
			return filepath.SkipDir
		} else {
			return nil
		}
	}

	// 以下正常時
	absPath, err := c.fs.Abs(path)
	if err != nil {
		fmt.Printf("Error: cannot get absolute path for %s\n", path)
		return nil
	}

	// 除外に一致するDirectoryはskipする
	excludeMatched := c.config.ExcludeRegexp.MatchString(absPath)
	if excludeMatched && info.IsDir() {
		return filepath.SkipDir
	}

	// 除外に一致しない通常ファイルは処理する
	if !excludeMatched && info.Type().IsRegular() {
		c.verboseLog("Check1: %s\n", absPath)

		if c.shouldDelete(absPath) {
			c.verboseLog("Check2: %s\n", absPath)

			// ファイルの年齢をチェック
			fileInfo, err := info.Info()
			if err != nil {
				fmt.Printf("Error: cannot get file info for %s\n", absPath)
				return nil
			}

			age := time.Since(fileInfo.ModTime())
			if age < c.config.MinAge {
				c.verboseLog("Skip: %s is too new (age: %s, min: %s)\n",
					absPath, age.Round(time.Hour), c.config.MinAge.Round(time.Hour))
				return nil
			}

			return c.handleFile(absPath, age)
		}
	}
	return nil
}

// shouldDelete checks if the file should be deleted based on its name
func (c *Cleaner) shouldDelete(path string) bool {
	return strings.HasSuffix(path, "~") || strings.HasSuffix(path, ".bak")
}

// handleFile handles the actual file deletion or dry run
func (c *Cleaner) handleFile(path string, age time.Duration) error {
	if c.config.DryRun {
		fmt.Printf("Would remove: %s (age: %s)\n", path, age.Round(time.Hour))
		return nil
	}

	// Interactive confirmation
	if c.config.Interactive {
		if !c.askConfirmation(fmt.Sprintf("Delete %s? (y/n): ", path)) {
			return nil
		}
	}

	if err := c.fs.Remove(path); err != nil {
		fmt.Printf("Error: cannot remove %s\n", path)
	} else {
		fmt.Printf("Removed: %s\n", path)
	}
	return nil
}

// askConfirmation asks for user confirmation
func (c *Cleaner) askConfirmation(message string) bool {
	fmt.Print(message)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// verboseLog prints if verbose mode is enabled
func (c *Cleaner) verboseLog(format string, args ...interface{}) {
	if c.config.Verbose {
		fmt.Printf(format, args...)
	}
}

func main() {
	config := NewConfig()

	if err := config.ParseFlags(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if config.ShowVersion {
		fmt.Println("Directory Sweeper ver", version)
		os.Exit(0)
	}

	fs := &RealFileSystem{}
	cleaner := NewCleaner(fs, config)

	if err := cleaner.Clean(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if config.Verbose {
		fmt.Println("Succeeded.")
	}
}
