package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var version = "0.2.1"

// vlogがtrueなら出力するPrintf
type verboseT bool

var vlog verboseT = false

func (d verboseT) Printf(format string, args ...interface{}) {
	if d {
		fmt.Printf(format, args...)
	}
}

// file scopeにするしかないのかなこいつら
var dryRun bool = false
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
		vlog.Printf("Check1: %s\n", path)
		if strings.HasSuffix(path, "~") {
			vlog.Printf("Check2: %s\n", path)
			if !dryRun {
				if os.Remove(path) != nil {
					fmt.Printf("Error: cannot remove %s\n", path)
				} else {
					vlog.Printf("Removed: %s\n", path)
				}
			}
		}
	}
	return nil
}

func main() {
	var showVersion bool
	var excludePattern string
	var verbose bool
	var dir string = "."
	
	flag.BoolVar(&dryRun, "n", false, "print filename but not delete")
	flag.BoolVar(&dryRun, "dryrun", false, "print filename but not delete")
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.StringVar(&excludePattern, "x", "\x00", "exclude path string")
	flag.StringVar(&excludePattern, "exclude", "\x00", "exclude path string")
	flag.BoolVar(&verbose, "verbose", false, "verbose")
	flag.Parse()

	vlog = verboseT(verbose) // もうちょっとよい手はないか?

	if excludePattern == "\x00" {
		excludePattern = ".*[\\/].elmo[\\/].*"
	}
	if excludePattern != "" {
		var err error
		excludeRegexp, err = regexp.Compile(excludePattern)
		if err != nil {
			fmt.Println("Illegal regexp.")
			os.Exit(1)
		}
		excludeExist = true
	}
	if showVersion {
		fmt.Println("Directory Sweeper ver", version)
		os.Exit(0)
	}

	if flag.NArg() >= 1 {
		dir = flag.Arg(0)
	}
	vlog.Printf("Exclude Pattern: %s\n", excludePattern)
	vlog.Printf("Target Directory: %s\n", dir)
	err := filepath.Walk(dir, sweep)
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
