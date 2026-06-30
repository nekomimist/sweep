package sweep

import (
	"fmt"
	"io"
)

const (
	ExitOK      = 0
	ExitRuntime = 1
	ExitUsage   = 2
)

// Run parses CLI arguments, runs the cleaner, and returns a process exit code.
func Run(args []string, input io.Reader, output io.Writer, errorOutput io.Writer) int {
	config, err := ParseConfig(args, errorOutput)
	if err != nil {
		fmt.Fprintf(errorOutput, "Error: %v\n", err)
		return ExitUsage
	}

	if config.ShowHelp {
		return ExitOK
	}
	if config.ShowVersion {
		fmt.Fprintln(output, "Directory Sweeper ver", Version)
		return ExitOK
	}

	cleaner := NewCleaner(RealFileSystem{}, config, input, output, errorOutput)
	result := cleaner.Clean()
	if result.Err() != nil {
		return ExitRuntime
	}

	if config.Verbose {
		fmt.Fprintln(output, "Succeeded.")
	}
	return ExitOK
}
