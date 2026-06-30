package sweep

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/spf13/pflag"
)

const (
	defaultDirectory      = "."
	defaultExcludePattern = `[\\/]\.elmo[\\/]`
)

// Version is the user-visible program version.
const Version = "0.2.2"

// Config holds all configuration options for the sweep command.
type Config struct {
	DryRun         bool
	Verbose        bool
	Interactive    bool
	Confirm        bool
	ShowVersion    bool
	ShowHelp       bool
	ExcludePattern string
	Directory      string
	MinAgeDays     int
	MinAge         time.Duration
	ExcludeRegexp  *regexp.Regexp
}

// NewConfig creates a new Config with default values.
func NewConfig() Config {
	return Config{
		Directory:      defaultDirectory,
		ExcludePattern: defaultExcludePattern,
	}
}

// ParseConfig parses command line flags into Config.
func ParseConfig(args []string, output io.Writer) (Config, error) {
	config := NewConfig()
	flags := newFlagSet(output, &config)

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			config.ShowHelp = true
			return config, nil
		}
		return config, err
	}

	if help, _ := flags.GetBool("help"); help {
		flags.Usage()
		config.ShowHelp = true
		return config, nil
	}

	if config.MinAgeDays < 0 {
		return config, fmt.Errorf("--age must be greater than or equal to 0")
	}
	config.MinAge = time.Duration(config.MinAgeDays) * 24 * time.Hour

	if flags.NArg() > 1 {
		return config, fmt.Errorf("expected at most one directory argument, got %d", flags.NArg())
	}
	if flags.NArg() == 1 {
		config.Directory = flags.Arg(0)
	}

	excludeRegexp, err := regexp.Compile(config.ExcludePattern)
	if err != nil {
		return config, fmt.Errorf("illegal regexp: %w", err)
	}
	config.ExcludeRegexp = excludeRegexp

	return config, nil
}

func newFlagSet(output io.Writer, config *Config) *pflag.FlagSet {
	flags := pflag.NewFlagSet("sweep", pflag.ContinueOnError)
	flags.SetOutput(output)
	flags.SortFlags = false
	flags.Usage = func() {
		fmt.Fprintf(output, "Usage of %s:\n", flags.Name())
		flags.PrintDefaults()
	}

	flags.BoolVarP(&config.DryRun, "dry-run", "n", false, "print filename but not delete")
	flags.BoolVarP(&config.ShowVersion, "version", "v", false, "show version")
	flags.StringVarP(&config.ExcludePattern, "exclude", "x", defaultExcludePattern, "exclude path regexp")
	flags.BoolVarP(&config.Verbose, "verbose", "V", false, "verbose output")
	flags.BoolVarP(&config.Interactive, "interactive", "i", false, "ask before deleting each file")
	flags.BoolVarP(&config.Confirm, "confirm", "c", false, "ask before starting deletion")
	flags.IntVarP(&config.MinAgeDays, "age", "a", 0, "minimum age in days before deletion (0 = delete immediately)")
	flags.BoolP("help", "h", false, "show this help message")

	return flags
}
