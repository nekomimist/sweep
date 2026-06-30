// Copyright 2017,2021 Hiroyuki Ishikura. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	"os"

	"github.com/nekomimist/sweep/internal/sweep"
)

func main() {
	os.Exit(sweep.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
