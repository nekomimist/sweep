package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var version = "0.2.0"

// file scopeにするしかないのかなこいつら
var dryRun bool = false
var verbose bool = false
var excludeExist bool = false
var excludeRegexp = regexp.MustCompile("")

// 通常ファイルかつ末尾が~なら削除を試みるWalkFunc
func sweep(path string, info os.FileInfo, err error) error {
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
	path, _ = filepath.Abs(path) // ファイル名しか来ない以上Absは必ず成功
	excludeMatched := false
	if excludeExist {
		excludeMatched = excludeRegexp.MatchString(path)
	}
	if !excludeMatched && info.Mode().IsRegular() {
		if verbose {
			fmt.Printf("Check1: %s\n", path)
		}
		if strings.HasSuffix(path, "~") {
			if verbose {
				fmt.Printf("Check2: %s\n", path)
			}
			if !dryRun {
				err := os.Remove(path)
				if err != nil {
					fmt.Printf("Error: cannot remove %s\n", path)
				} else if verbose {
					fmt.Printf("Removed: %s\n", path)
				}
			}
		}
	}
	return nil
}

func main() {
	var showVersion bool
	var excludePattern string = ""

	dir := "."
	flag.BoolVar(&dryRun, "n", false, "print filename but not delete")
	flag.BoolVar(&dryRun, "dryrun", false, "print filename but not delete")
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&verbose, "verbose", false, "verbose")
	flag.StringVar(&excludePattern, "x", "", "exclude path string")
	flag.StringVar(&excludePattern, "exclude", "", "exclude path string")
	flag.Parse()

	if excludePattern != "" {
		var err error
		excludeRegexp, err = regexp.Compile(excludePattern)
		if err != nil {
			os.Exit(1)
		}
		excludeExist = true
	}
	if showVersion {
		fmt.Println("Directory Sweeper ver", version)
		os.Exit(0)
	}
	if flag.NArg() > 1 {
		dir = flag.Arg(1)
	}
	if verbose {
		fmt.Println("Exclude Pattern: ", excludePattern)
		fmt.Println("Target Directory: ", dir)
	}
	err := filepath.Walk(dir, sweep)
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
