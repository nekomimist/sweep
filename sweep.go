package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var version = "0.1.0"
var dryRun bool = false

// 通常ファイルかつ末尾が~なら削除を試みるWalkFunc
func sweep(path string, info os.FileInfo, err error) error {
	if err != nil {
		fmt.Printf("Error: skip -- %s\n", path)
		return nil
	}
	if info.Mode().IsRegular() { // 通常ファイル以外は対象外
		if !dryRun && strings.HasSuffix(path, "~") {
			fmt.Println(path)
			err := os.Remove(path)
			if err != nil {
				fmt.Printf("Error: cannot remove -- %s\n", path)
			}
		}
	}
	return nil
}

func main() {
	var showVersion bool
	dir := "."
	flag.BoolVar(&dryRun, "n", false, "print filename but not delete")
	flag.BoolVar(&dryRun, "dry-run", false, "print filename but not delete")
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.Parse()

	if showVersion {
		fmt.Println("version:", version)
		return
	}
	if flag.NArg() > 1 {
		dir = flag.Arg(1)
	}
	filepath.Walk(dir, sweep)
}
