// Copyright 2017 Hiroyuki Ishikura. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const version = "0.2.1"

// vlogがtrueなら出力するPrintf
type verboseT bool

var vlog verboseT = false

func (d verboseT) Printf(format string, args ...interface{}) {
	if d {
		fmt.Printf(format, args...)
	}
}

// 通常ファイルかつ末尾が~なら削除を試みるWalkFunc
func sweepFunc(dryRun bool, regexp *regexp.Regexp) filepath.WalkFunc {
	// WalkFunc用の引数を外から貰うためにこの形
	return func(path string, info os.FileInfo, err error) error {
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
		path, _ = filepath.Abs(path)
		// Go1.8まではWindowsのDirectory JunctionもWalkしてしまう
		if info.IsDir() && info.Mode()&os.ModeSymlink == os.ModeSymlink {
			vlog.Printf("Skip : Directory Junction %s\n", path)
			return filepath.SkipDir
		}
		// 除外に一致するDirectoryはskipする
		excludeMatched := regexp.MatchString(path)
		if excludeMatched && info.IsDir() {
			return filepath.SkipDir
		}
		// 除外に一致しない通常ファイルは処理する
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
}

func main() {
	var showVersion bool
	var excludePattern string
	var dir string = "."
	var dryRun bool

	flag.BoolVar(&dryRun, "n", false, "print filename but not delete")
	flag.BoolVar(&dryRun, "dryrun", false, "print filename but not delete")
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.StringVar(&excludePattern, "x", "\x00", "exclude path regexp")
	flag.StringVar(&excludePattern, "exclude", "\x00", "exclude path regexp")
	flag.BoolVar((*bool)(&vlog), "verbose", false, "verbose")
	flag.Parse()

	if excludePattern == "\x00" { // 定義ファイルから読みたいところ
		excludePattern = `[\\/]\.elmo[\\/]`
	}
	excludeRegexp, err := regexp.Compile(excludePattern)
	if err != nil {
		fmt.Println("Illegal regexp.")
		os.Exit(1)
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
	if filepath.Walk(dir, sweepFunc(dryRun, excludeRegexp)) != nil {
		vlog.Printf("Failed.\n")
		os.Exit(1)
	} else {
		vlog.Printf("Succeeded.\n")
		os.Exit(0)
	}
}
